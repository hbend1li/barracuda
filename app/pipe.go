package app

import (
	"encoding/gob"
	"log"
	"net"
	"os"
	"sync"

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

// Socket-related, server-related functions

func createOpenSocket() net.Listener {
	socketPath := SocketPath()
	_, err := os.Stat(socketPath)
	if err == nil {
		log.Println("WARN  socket", socketPath, "already exists: Is the daemon already running? Deleting.")
		err = os.Remove(socketPath)
		if err != nil {
			log.Println("FATAL Failed to remove socket:", err)
		}
	}
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Println("FATAL Failed to create socket:", err)
	}
	return ln
}

// Handle connections
func Serve() {
	ln := createOpenSocket()
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("ERROR Failed to open connection from cli:", err)
			continue
		}
		go func(conn net.Conn) {
			var request Request
			var response Response

			err := gob.NewDecoder(conn).Decode(&request)
			if err != nil {
				log.Println("ERROR Invalid Message from cli:", err)
				return
			}

			switch request.Request {
			case Query:
				response.Actions = actionStore.store.ToReadable()
			case Flush:
				actionStore.Flush(request.Pattern)
			default:
				log.Println("ERROR Invalid Message from cli: unrecognised Request type")
				return
			}

			gob.NewEncoder(conn).Encode(response)
			if err != nil {
				log.Println("ERROR Can't respond to cli:", err)
				return
			}
		}(conn)
	}
}
