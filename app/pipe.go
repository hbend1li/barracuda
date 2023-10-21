package app

import (
	"encoding/gob"
	"net"
	"os"
	"path"
	"sync"
	"time"

	"framagit.org/ppom/reaction/logger"
)

func genClientStatus(local_actions ActionsMap, local_matches MatchesMap, local_actionsLock, local_matchesLock *sync.Mutex) ClientStatus {
	cs := make(ClientStatus)
	local_matchesLock.Lock()

	// Painful data manipulation
	for pf, times := range local_matches {
		pattern, filter := pf.p, pf.f
		if cs[filter.stream.name] == nil {
			cs[filter.stream.name] = make(map[string]MapPatternStatus)
		}
		if cs[filter.stream.name][filter.name] == nil {
			cs[filter.stream.name][filter.name] = make(MapPatternStatus)
		}
		cs[filter.stream.name][filter.name][pattern] = &PatternStatus{len(times), nil}
	}

	local_matchesLock.Unlock()
	local_actionsLock.Lock()

	// Painful data manipulation
	for pa, times := range local_actions {
		pattern, action := pa.p, pa.a
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
		for then := range times {
			ps.Actions[action.name] = append(ps.Actions[action.name], then.Format(time.DateTime))
		}
	}
	local_actionsLock.Unlock()
	return cs
}

func createOpenSocket() net.Listener {
	err := os.MkdirAll(path.Dir(*SocketPath), 0755)
	if err != nil {
		logger.Fatalln("Failed to create socket directory")
	}
	_, err = os.Stat(*SocketPath)
	if err == nil {
		logger.Println(logger.WARN, "socket", SocketPath, "already exists: Is the daemon already running? Deleting.")
		err = os.Remove(*SocketPath)
		if err != nil {
			logger.Fatalln("Failed to remove socket:", err)
		}
	}
	ln, err := net.Listen("unix", *SocketPath)
	if err != nil {
		logger.Fatalln("Failed to create socket:", err)
	}
	return ln
}

// Handle connections
func SocketManager(streams map[string]*Stream) {
	ln := createOpenSocket()
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			logger.Println(logger.ERROR, "Failed to open connection from cli:", err)
			continue
		}
		go func(conn net.Conn) {
			defer conn.Close()
			var request Request
			var response Response

			err := gob.NewDecoder(conn).Decode(&request)
			if err != nil {
				logger.Println(logger.ERROR, "Invalid Message from cli:", err)
				return
			}

			switch request.Request {
			case Show:
				response.ClientStatus = genClientStatus(actions, matches, &actionsLock, &matchesLock)
			case Flush:
				le := LogEntry{time.Now(), 0, request.Pattern, "", "", 0, false}
				matchesC := FlushMatchOrder{request.Pattern, make(chan MatchesMap)}
				actionsC := FlushActionOrder{request.Pattern, make(chan ActionsMap)}
				flushToMatchesC <- matchesC
				flushToActionsC <- actionsC
				flushToDatabaseC <- le

				var lock sync.Mutex
				response.ClientStatus = genClientStatus(<-actionsC.ret, <-matchesC.ret, &lock, &lock)
			default:
				logger.Println(logger.ERROR, "Invalid Message from cli: unrecognised Request type")
				return
			}

			err = gob.NewEncoder(conn).Encode(response)
			if err != nil {
				logger.Println(logger.ERROR, "Can't respond to cli:", err)
				return
			}
		}(conn)
	}
}
