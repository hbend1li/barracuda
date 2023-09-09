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
	logDBName    = "./reaction-matches.db"
	logDBNewName = "./reaction-matches.new.db"
	flushDBName  = "./reaction-flushes.db"
)

type ReadDB struct {
	file *os.File
	dec  *gob.Decoder
}

type WriteDB struct {
	file *os.File
	enc  *gob.Encoder
}

func openDB(path string) (bool, *ReadDB) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("WARN  No DB found at %s. It's ok if this is the first time reaction is running.\n", path)
			return true, nil
		}
		log.Fatalln("FATAL Failed to open DB:", err)
	}
	return false, &ReadDB{file, gob.NewDecoder(file)}
}

func createDB(path string) *WriteDB {
	file, err := os.Create(path)
	if err != nil {
		log.Fatalln("FATAL Failed to create DB:", err)
	}
	return &WriteDB{file, gob.NewEncoder(file)}
}

func (c *Conf) DatabaseManager() {
	logDB, flushDB := c.RotateDB(true)
	log.Print("DEBUG startup finished, managing logs!")
	c.manageLogs(logDB, flushDB)
}

func (c *Conf) manageLogs(logDB *WriteDB, flushDB *WriteDB) {
	var cpt int
	for {
		select {
		case entry := <-flushes:
			flushDB.enc.Encode(entry)
		case entry := <-logs:
			logDB.enc.Encode(entry)
			cpt++
			// let's say 100 000 entries ~ 10 MB
			if cpt == 100_000 {
				cpt = 0
				log.Printf("INFO  Rotating database...")
				logDB.file.Close()
				flushDB.file.Close()
				logDB, flushDB = c.RotateDB(false)
				log.Printf("INFO  Rotated database")
			}
		}
	}
}

func (c *Conf) RotateDB(startup bool) (*WriteDB, *WriteDB) {
	var (
		doesntExist  bool
		err          error
		logReadDB    *ReadDB
		flushReadDB  *ReadDB
		logWriteDB   *WriteDB
		flushWriteDB *WriteDB
	)
	doesntExist, logReadDB = openDB(logDBName)
	if doesntExist {
		return createDB(logDBName), createDB(flushDBName)
	}
	doesntExist, flushReadDB = openDB(flushDBName)
	if doesntExist {
		log.Println("WARN  Strange! No flushes db, opening /dev/null instead")
		doesntExist, flushReadDB = openDB("/dev/null")
		if doesntExist {
			log.Fatalln("Opening dummy /dev/null failed")
		}
	}

	logWriteDB = createDB(logDBNewName)

	rotateDB(c, logReadDB.dec, flushReadDB.dec, logWriteDB.enc, startup)

	err = logReadDB.file.Close()
	if err != nil {
		log.Fatalln("FATAL Failed to close old DB:", err)
	}

	// It should be ok to rename an open file
	err = os.Rename(logDBNewName, logDBName)
	if err != nil {
		log.Fatalln("FATAL Failed to replace old DB with new one:", err)
	}

	err = os.Remove(flushDBName)
	if err != nil {
		log.Fatalln("FATAL Failed to delete old DB:", err)
	}

	flushWriteDB = createDB(flushDBName)
	return logWriteDB, flushWriteDB
}

func rotateDB(c *Conf, logDec *gob.Decoder, flushDec *gob.Decoder, logEnc *gob.Encoder, startup bool) {
	log.Println("DEBUG rotating db")
	// This extra code is made to warn only one time for each non-existant filter
	type SF struct{ s, f string }
	discardedEntries := make(map[SF]int)
	malformedEntries := 0
	defer func() {
		for sf, t := range discardedEntries {
			if t > 0 {
				log.Printf("WARN  info discarded %v times from the DBs: stream/filter not found: %s.%s\n", t, sf.s, sf.f)
			}
		}
		if malformedEntries > 0 {
			log.Printf("WARN  %v malformed entries discarded from the DBs\n", malformedEntries)
		}
	}()

	var err error
	var entry LogEntry
	var filter *Filter

	type PSF struct{ p, s, f string }

	// pattern, stream, fitler → last flush
	flushes := make(map[PSF]time.Time)
	for {
		// decode entry
		err = flushDec.Decode(&entry)
		if err != nil {
			if err == io.EOF {
				break
			}
			malformedEntries++
			continue
		}

		// retrieve related filter
		if entry.Stream != "" || entry.Filter != "" {
			if stream := c.Streams[entry.Stream]; stream != nil {
				filter = stream.Filters[entry.Filter]
			}
			if filter == nil {
				discardedEntries[SF{entry.Stream, entry.Filter}]++
				continue
			}
		}

		// store
		log.Printf("DEBUG got flush: %v", entry.Pattern)
		flushes[PSF{entry.Pattern, entry.Stream, entry.Filter}] = entry.T
	}
	log.Printf("DEBUG flushes: %v", flushes)

	now := time.Now()
	for {

		// decode entry
		err = logDec.Decode(&entry)
		if err != nil {
			if err == io.EOF {
				break
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

		// check if it hasn't been flushed, only for Exec:true for now
		if entry.Exec {
			lastGlobalFlush := flushes[PSF{entry.Pattern, "", ""}].Unix()
			lastLocalFlush := flushes[PSF{entry.Pattern, entry.Stream, entry.Filter}].Unix()
			entryTime := entry.T.Unix()

			if lastLocalFlush > entryTime || lastGlobalFlush > entryTime {
				log.Printf("DEBUG got %v (exec:%v) but it has been flushed\n", entry.Pattern, entry.Exec)
				continue
			}
		}

		// store matches
		if !entry.Exec && entry.T.Add(filter.retryDuration).Unix() > now.Unix() {
			log.Printf("DEBUG got match: %v\n", entry.Pattern)
			if startup {
				filter.matches[entry.Pattern] = append(filter.matches[entry.Pattern], entry.T)
			}

			encodeOrFatal(logEnc, entry)
		}

		// replay executions
		if entry.Exec && entry.T.Add(*filter.longuestActionDuration).Unix() > now.Unix() {
			log.Printf("DEBUG got exec: %v\n", entry.Pattern)
			if startup {
				delete(filter.matches, entry.Pattern)
				filter.execActions(entry.Pattern, now.Sub(entry.T))
			}

			encodeOrFatal(logEnc, entry)
		}
	}
}

func encodeOrFatal(enc *gob.Encoder, entry LogEntry) {
	err := enc.Encode(entry)
	if err != nil {
		log.Fatalln("FATAL Failed to write to new DB:", err)
	}
}
