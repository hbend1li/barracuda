package app

import (
	"encoding/json"
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

// Append match, remove old ones and check match number
type AppendMatch struct {
	filter  *Filter
	pattern *string
	t       time.Time
}

var env *lmdb.Env

// e == "this match triggers an execution of the commands of the related filter"
type MatchItem struct {
	t time.Time
	e bool
}

type PatternMatchItem struct {
	pattern    string
	matchItems []MatchItem
}

// keys: pattern
// values: []struct { time.Time, bool triggerExec }
var matchDBs map[*Filter]lmdb.DBI

func dbGet(dbi lmdb.DBI, pattern string) ([]MatchItem, error) {
	err = env.View(func(txn *lmdb.Txn) (err error) {
		v, err := txn.Get(dbi, []byte(pattern))
		if err != nil {
			return nil, err
		}
		value := make([]MatchItem)
		err := json.Unmarshal(v, &value)
		return value, err
	})
}

func dbPut(dbi lmdb.DBI, pattern string, value []MatchItem) error {
	err = env.UpdateLocked(func(txn *lmdb.Txn) (err error) {
		jsonValue := json.Marshal(value)
		return txn.Put(dbi, []byte(pattern), jsonValue, 0)
	})
}

func dbDel(dbi lmdb.DBI, pattern string) error {
	err = env.UpdateLocked(func(txn *lmdb.Txn) (err error) {
		return txn.Del(dbi, []byte(pattern), []byte())
	})
}

func dbList(dbi lmdb.DBI) (chan []PatternMatchItem, chan error) {
	items := make(chan []PatternMatchItem)
	errors := make(chan []error)

	go func() {

		err = env.View(func(txn *lmdb.Txn) (err error) {
			cur, err := txn.OpenCursor(dbi)
			if err != nil {
				return err
			}
			defer cur.Close()

			for {
				k, v, err := cur.Get(nil, nil, lmdb.Next)
				if lmdb.IsNotFound(err) {
					return nil
				}
				if err != nil {
					return err
				}

				value := make([]MatchItem)
				err = json.Unmarshal(v, &value)
				if err != nil {
					return err
				}

				items <- PatternMatchItem{
					pattern:    string(k),
					matchItems: value,
				}
			}
		})
	}()

	return items, errors
}

// For now it's a list.
// With a CLI, it might be useful to make a
// map [*Filter]map[pattern]chan bool
// if have a way to configure how reaction handles persistance,
// bool would then mean if we execute or quit without executing
var execActions []chan bool

func databaseHandler(matches chan AppendMatch) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer env.Close()
	defer func() {
		for action := range execActions {
			action <- true
		}
	}()

	for filter, dbi := range matchDBs {
		items, errs := dbList(dbi)
		select {
		case err := <-errs:
			// warn user & delete entry if unmarshal failed?
			// or panic, and tell user to delete db?
		default:
			select {
			case item := <-items:
				// check if last exec is not too old (now < lastExec + longuestCommand"After")
				if true {
					for command := range filter.commands {
						// launch command
					}
				}
			default:
			}
		}
	}

	select {
	case am := <-chAM:
		currentMatches := dbGet(matchDBs[am.filter], am.pattern)
		// add match
		// check if enough watches in interval
		if true {
			for command := range am.filter.commands {
				// launch command
			}
		}

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

	err = env.SetMaxDBs(filterNumber)
	if err != nil {
		log.Fatalln("LMDB.SetMaxDBs failed")
	}

	matchDBs = make(map[*Filter]lmdb.DBI, filterNumber)

	runtime.LockOSThread()

	for _, stream := range conf.Streams {
		for _, filter := range stream.Filters {
			err = env.UpdateLocked(func(txn *lmdb.Txn) (err error) {
				matchDBs[filter], err = txn.CreateDBI(fmt.Sprintln("%s.%s", stream.name, filter.name))
				return err
			})
			if err != nil {
				log.Fatalln("LMDB.CreateDBI failed")
			}
		}
	}

	runtime.UnlockOSThread()

	chAM := make(chan AppendMatch)

	go databaseHandler(chAM)

	return chAM
}
