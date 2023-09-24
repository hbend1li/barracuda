package app

import (
	"encoding/gob"
	"log"
	"net"
	"os"
	"path"
	"time"
)

func genClientStatus() ClientStatus {
	cs := make(ClientStatus)
	matchesLock.Lock()

	// Painful data manipulation
	for pf, times := range matches {
		pattern, filter := pf.p, pf.f
		if cs[filter.stream.name] == nil {
			cs[filter.stream.name] = make(map[string]MapPatternStatus)
		}
		if cs[filter.stream.name][filter.name] == nil {
			cs[filter.stream.name][filter.name] = make(MapPatternStatus)
		}
		cs[filter.stream.name][filter.name][pattern] = &PatternStatus{len(times), nil}
	}

	matchesLock.Unlock()
	actionsLock.Lock()

	// Painful data manipulation
	for pat := range actions {
		pattern, action, then := pat.p, pat.a, pat.t
		if cs[action.filter.stream.name] == nil {
			cs[action.filter.stream.name] = make(map[string]MapPatternStatus)
		}
		if cs[action.filter.stream.name][action.filter.name] == nil {
			cs[action.filter.stream.name][action.filter.name] = make(MapPatternStatus)
		}
		if cs[action.filter.stream.name][action.filter.name][pattern] == nil {
			cs[action.filter.stream.name][action.filter.name][pattern] = new(PatternStatus)
		}
		ps := cs[action.filter.stream.name][action.filter.name][pattern]
		if ps.Actions == nil {
			ps.Actions = make(map[string][]string)
		}
		ps.Actions[action.name] = append(ps.Actions[action.name], then.Format(time.DateTime))
	}
	actionsLock.Unlock()
	return cs
}

func createOpenSocket() net.Listener {
	err := os.MkdirAll(path.Dir(*SocketPath), 0755)
	if err != nil {
		log.Fatalln("FATAL Failed to create socket directory")
	}
	_, err = os.Stat(*SocketPath)
	if err == nil {
		log.Println("WARN  socket", SocketPath, "already exists: Is the daemon already running? Deleting.")
		err = os.Remove(*SocketPath)
		if err != nil {
			log.Fatalln("FATAL Failed to remove socket:", err)
		}
	}
	ln, err := net.Listen("unix", *SocketPath)
	if err != nil {
		log.Fatalln("FATAL Failed to create socket:", err)
	}
	return ln
}

// Handle connections
func SocketManager() {
	ln := createOpenSocket()
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("ERROR Failed to open connection from cli:", err)
			continue
		}
		go func(conn net.Conn) {
			defer conn.Close()
			var request Request
			var response Response

			err := gob.NewDecoder(conn).Decode(&request)
			if err != nil {
				log.Println("ERROR Invalid Message from cli:", err)
				return
			}

			switch request.Request {
			case Show:
				response.ClientStatus = genClientStatus()
			case Flush:
				// FIXME reimplement flush
				response.Number = 0
			default:
				log.Println("ERROR Invalid Message from cli: unrecognised Request type")
				return
			}

			err = gob.NewEncoder(conn).Encode(response)
			if err != nil {
				log.Println("ERROR Can't respond to cli:", err)
				return
			}
		}(conn)
	}
}
