package app

import (
	"encoding/gob"
	"errors"
	"log"
	"os"
	"sync"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

type ActionMap map[string]map[*Action]map[chan bool]bool
type ReadableMap map[string]map[string]map[string]int

type ActionStore struct {
	store ActionMap
	mutex sync.Mutex
}

// Called by an Action before entering sleep
func (a *ActionStore) Register(action *Action, pattern string) chan bool {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	if a.store[pattern] == nil {
		a.store[pattern] = make(map[*Action]map[chan bool]bool)
	}
	if a.store[pattern][action] == nil {
		a.store[pattern][action] = make(map[chan bool]bool)
	}
	sig := make(chan bool)
	a.store[pattern][action][sig] = true
	return sig
}

// Called by an Action after sleep
func (a *ActionStore) Unregister(action *Action, pattern string, sig chan bool) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	if a.store[pattern] == nil || a.store[pattern][action] == nil || len(a.store[pattern][action]) == 0 {
		return
	}
	close(sig)
	delete(a.store[pattern][action], sig)
}

// Called by Main
func (a *ActionStore) Quit() {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	for _, actions := range a.store {
		for _, sigs := range actions {
			for sig := range sigs {
				close(sig)
			}
		}
	}
	a.store = make(ActionMap)
}

// Called by a CLI
func (a *ActionStore) Flush(pattern string) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	if a.store[pattern] != nil {
		for _, action := range a.store[pattern] {
			for sig := range action {
				close(sig)
			}
		}
	}
	delete(a.store, pattern)
}

// Called by a CLI
func (a *ActionStore) pendingActions() ReadableMap {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	return a.store.ToReadable()
}

func (a ActionMap) ToReadable() ReadableMap {
	res := make(ReadableMap)

	for pattern, actions := range a {
		for action := range actions {
			filter := action.filter.name
			stream := action.filter.stream.name
			if res[stream] == nil {
				res[stream] = make(map[string]map[string]int)
			}
			if res[stream][filter] == nil {
				res[stream][filter] = make(map[string]int)
			}
			res[stream][filter][pattern] = res[stream][filter][pattern] + 1
		}
	}

	return res
}

func (r ReadableMap) ToString() string {
	text, err := yaml.Marshal(r)
	if err != nil {
		log.Fatalln(err)
	}
	return string(text)
}

// Pipe-related, server-related functions

func createOpenPipe() *os.File {
	err := os.Mkdir(RuntimeDirectory(), 0755)
	if err != nil && !errors.Is(err, os.ErrExist) {
		log.Fatalln("FATAL Failed to create runtime directory", err)
	}
	pipePath := PipePath()
	_, err = os.Stat(pipePath)
	if err == nil {
		log.Println("WARN  Runtime file", pipePath, "already exists: Is the daemon already running? Deleting.")
		err = os.Remove(pipePath)
		if err != nil {
			log.Println("FATAL Failed to remove runtime file:", err)
		}
	}
	err = syscall.Mkfifo(pipePath, 0600)
	if err != nil {
		log.Println("FATAL Failed to create runtime file:", err)
	}
	file, err := os.OpenFile(pipePath, os.O_RDONLY, os.ModeNamedPipe)
	if err != nil {
		log.Println("FATAL Failed to open runtime file:", err)
	}
	return file
}

func Respond(request Request, response Response) {
	file, err := os.Create(request.ResponsePath())
	if err != nil {
		log.Println("WARN  Can't respond to message:", err)
		return
	}
	err = gob.NewEncoder(file).Encode(response)
	if err != nil {
		log.Println("WARN  Can't respond to message:", err)
		return
	}
}

// Handle connections
func Serve() {
	pipe := createOpenPipe()
	for {
		var request Request
		err := gob.NewDecoder(pipe).Decode(&request)
		if err != nil {
			d, _ := time.ParseDuration("1s")
			if err.Error() == "EOF" {
				log.Println("DEBUG received EOF, seeking one byte")
				_, err = pipe.Seek(1, 1)
				if err != nil {
					log.Println("DEBUG failed to seek:", err)
				}
				time.Sleep(d)
				continue
			}
			log.Println("WARN  Invalid Message received:", err)
			time.Sleep(d)
			continue
		}
		go func(request Request) {
			var response Response
			switch request.Request {
			case Query:
				response.Actions = actionStore.store.ToReadable()
			case Flush:
				actionStore.Flush(request.Pattern)
			default:
				log.Println("WARN  Invalid Message: unrecognised Request type")
			}
			Respond(request, response)
		}(request)
	}
}
