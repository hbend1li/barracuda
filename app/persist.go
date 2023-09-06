package app

import (
	"encoding/gob"
	"errors"
	"io"
	"log"
	"os"
	"time"
)

const (
	dbName    = "./reaction.db"
	dbNewName = "./reaction.new.db"
)

type ReadDB struct {
	file *os.File
	dec  *gob.Decoder
}

type WriteDB struct {
	file *os.File
	enc  *gob.Encoder
}

func openDB(path string) (error, *ReadDB) {
	file, err := os.Open(path)
	if err != nil {
		return err, nil
	}
	return nil, &ReadDB{file, gob.NewDecoder(file)}
}

func createDB(path string) (error, *WriteDB) {
	file, err := os.Create(path)
	if err != nil {
		return err, nil
	}
	return nil, &WriteDB{file, gob.NewEncoder(file)}
}

func (c *Conf) DatabaseManager() chan *LogEntry {
	logs := make(chan *LogEntry)
	go func() {
		db := c.RotateDB(true)
		go c.manageLogs(logs, db)
	}()
	return logs
}

func (c *Conf) manageLogs(logs <-chan *LogEntry, db *WriteDB) {
	var cpt int
	for {
		db.enc.Encode(<-logs)
		cpt++
		// let's say 100 000 entries ~ 10 MB
		if cpt == 100_000 {
			cpt = 0
			db.file.Close()
			log.Printf("INFO  Rotating database...")
			db = c.RotateDB(false)
			log.Printf("INFO  Rotated database")
		}
	}
}

func (c *Conf) RotateDB(startup bool) *WriteDB {
	var (
		err error
		enc *WriteDB
		dec *ReadDB
	)
	err, dec = openDB(dbName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("WARN  No DB found at %s. It's ok if this is the first time reaction is running.\n", dbName)
			err, enc := createDB(dbName)
			if err != nil {
				log.Fatalln("FATAL Failed to create DB:", err)
			}
			return enc
		}
		log.Fatalln("FATAL Failed to open DB:", err)
	}

	err, enc = createDB(dbNewName)
	if err != nil {
		log.Fatalln("FATAL Failed to create new DB:", err)
	}

	rotateDB(c, dec.dec, enc.enc, startup)

	err = dec.file.Close()
	if err != nil {
		log.Fatalln("FATAL Failed to close old DB:", err)
	}

	// It should be ok to rename an open file
	err = os.Rename(dbNewName, dbName)
	if err != nil {
		log.Fatalln("FATAL Failed to replace old DB with new one:", err)
	}

	return enc
}

func rotateDB(c *Conf, dec *gob.Decoder, enc *gob.Encoder, startup bool) {
	// This extra code is made to warn only one time for each non-existant filter
	type SF struct{ s, f string }
	discardedEntries := make(map[SF]int)
	malformedEntries := 0
	defer func() {
		for sf, t := range discardedEntries {
			if t > 0 {
				log.Printf("WARN  info discarded %v times from the DB: stream/filter not found: %s.%s\n", t, sf.s, sf.f)
			}
		}
		if malformedEntries > 0 {
			log.Printf("WARN  %v malformed entries discarded from the DB\n", malformedEntries)
		}
	}()

	encodeOrFatal := func(entry LogEntry) {
		err := enc.Encode(entry)
		if err != nil {
			log.Fatalln("FATAL Failed to write to new DB:", err)
		}
	}

	var err error
	now := time.Now()
	for {
		var entry LogEntry
		var filter *Filter

		// decode entry
		err = dec.Decode(&entry)
		if err != nil {
			if err == io.EOF {
				return
			}
			malformedEntries++
			continue
		}

		// retrieve related filter
		if stream := c.Streams[entry.Stream]; stream != nil {
			filter = stream.Filters[entry.Filter]
		}
		if filter == nil {
			discardedEntries[SF{entry.Stream, entry.Filter}]++
			continue
		}

		// store matches
		if !entry.Exec && entry.T.Add(filter.retryDuration).Unix() > now.Unix() {
			if startup {
				filter.matches[entry.Pattern] = append(filter.matches[entry.Pattern], entry.T)
			}

			encodeOrFatal(entry)
		}

		// replay executions
		if entry.Exec && entry.T.Add(*filter.longuestActionDuration).Unix() > now.Unix() {
			if startup {
				delete(filter.matches, entry.Pattern)
				filter.execActions(entry.Pattern, now.Sub(entry.T))
			}

			encodeOrFatal(entry)
		}
	}
}
