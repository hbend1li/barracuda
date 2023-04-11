package app

import (
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/bmatsuo/lmdb-go/lmdb"
)

func numberOfFilters(conf *Conf) int {
	n := 0
	for _, s := range conf.Streams {
		n += len(s.Filters)
	}
	return n
}

type CmdTime struct {
	cmd []string
	t   time.Time
}

// Remove Cmd if last of its set
type CmdExecuted struct {
	filter  *Filter
	pattern *string
	value   CmdTime
	err     chan error
}

// Append Cmd set
type AppendCmd struct {
	filter  *Filter
	pattern *string
	value   []CmdTime
	err     chan error
}

// Append match, remove old ones and check match number
type AppendMatch struct {
	filter  *Filter
	pattern *string
	t       time.Time
	ret     chan struct {
		shouldExec bool
		err        error
	}
}

func databaseHandler(env *lmdb.Env, chCE chan CmdExecuted, chAC chan AppendCmd, chAM chan AppendMatch) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer env.Close()

	select {
	case ce := <-chCE:
		ce = ce
		// TODO
	case ac := <-chAC:
		ac = ac
		// TODO
	case am := <-chAM:
		am = am
		// TODO
	}
}

func initDatabase(conf *Conf) (chan CmdExecuted, chan AppendCmd, chan AppendMatch) {
	env, err := lmdb.NewEnv()
	if err != nil {
		log.Fatalln("LMDB.NewEnv failed")
	}

	err = env.SetMapSize(1 << 30)
	if err != nil {
		log.Fatalln("LMDB.SetMapSize failed")
	}

	filterNumber := numberOfFilters(conf)

	err = env.SetMaxDBs(filterNumber * 2)
	if err != nil {
		log.Fatalln("LMDB.SetMaxDBs failed")
	}

	matchDBs := make(map[*Filter]lmdb.DBI, filterNumber)
	cmdDBs := make(map[*Filter]lmdb.DBI, filterNumber)

	runtime.LockOSThread()

	for _, stream := range conf.Streams {
		for _, filter := range stream.Filters {
			err = env.UpdateLocked(func(txn *lmdb.Txn) (err error) {
				matchDBs[filter], err = txn.CreateDBI(fmt.Sprintln("%s.%s.match", stream.name, filter.name))
				if err != nil {
					return err
				}

				cmdDBs[filter], err = txn.CreateDBI(fmt.Sprintln("%s.%s.cmd", stream.name, filter.name))
				return err
			})
			if err != nil {
				log.Fatalln("LMDB.CreateDBI failed")
			}
		}
	}

	runtime.UnlockOSThread()

	chCE := make(chan CmdExecuted)
	chAC := make(chan AppendCmd)
	chAM := make(chan AppendMatch)

	go databaseHandler(env, chCE, chAC, chAM)

	return chCE, chAC, chAM
}
