package app

import (
	"encoding/gob"
	"errors"
	"io"
	"os"
	"time"

	"framagit.org/ppom/reaction/logger"
)

const (
	logDBName    = "./reaction-matches.db"
	logDBNewName = "./reaction-matches.new.db"
	flushDBName  = "./reaction-flushes.db"
)

func openDB(path string) (bool, *ReadDB) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logger.Printf(logger.WARN, "No DB found at %s. It's ok if this is the first time reaction is running.\n", path)
			return true, nil
		}
		logger.Fatalln("Failed to open DB:", err)
	}
	return false, &ReadDB{file, gob.NewDecoder(file)}
}

func createDB(path string) *WriteDB {
	file, err := os.Create(path)
	if err != nil {
		logger.Fatalln("Failed to create DB:", err)
	}
	return &WriteDB{file, gob.NewEncoder(file)}
}

func DatabaseManager(c *Conf) {
	logDB, flushDB := c.RotateDB(true)
	close(startupMatchesC)
	c.manageLogs(logDB, flushDB)
}

func (c *Conf) manageLogs(logDB *WriteDB, flushDB *WriteDB) {
	cpt := 0
	writeSF2int := make(map[SF]int)
	writeCpt := 1
	for {
		select {
		case entry := <-flushToDatabaseC:
			flushDB.enc.Encode(entry)
		case entry := <-logsC:
			encodeOrFatal(logDB.enc, entry, writeSF2int, &writeCpt)
			cpt++
			// let's say 100 000 entries ~ 10 MB
			if cpt == 500_000 {
				cpt = 0
				logger.Printf(logger.INFO, "Rotating database...")
				logDB.file.Close()
				flushDB.file.Close()
				logDB, flushDB = c.RotateDB(false)
				logger.Printf(logger.INFO, "Rotated database")
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
		logger.Println(logger.WARN, "Strange! No flushes db, opening /dev/null instead")
		doesntExist, flushReadDB = openDB("/dev/null")
		if doesntExist {
			logger.Fatalln("Opening dummy /dev/null failed")
		}
	}

	logWriteDB = createDB(logDBNewName)

	rotateDB(c, logReadDB.dec, flushReadDB.dec, logWriteDB.enc, startup)

	err = logReadDB.file.Close()
	if err != nil {
		logger.Fatalln("Failed to close old DB:", err)
	}

	// It should be ok to rename an open file
	err = os.Rename(logDBNewName, logDBName)
	if err != nil {
		logger.Fatalln("Failed to replace old DB with new one:", err)
	}

	err = os.Remove(flushDBName)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.Fatalln("Failed to delete old DB:", err)
	}

	flushWriteDB = createDB(flushDBName)
	return logWriteDB, flushWriteDB
}

func rotateDB(c *Conf, logDec *gob.Decoder, flushDec *gob.Decoder, logEnc *gob.Encoder, startup bool) {
	// This mapping is a space optimization feature
	// It permits to compress stream+filter to a small number (which is a byte in gob)
	// We do this only for matches, not for flushes
	readSF2int := make(map[int]SF)
	writeSF2int := make(map[SF]int)
	writeCounter := 1
	// This extra code is made to warn only one time for each non-existant filter
	discardedEntries := make(map[SF]int)
	malformedEntries := 0
	defer func() {
		for sf, t := range discardedEntries {
			if t > 0 {
				logger.Printf(logger.WARN, "info discarded %v times from the DBs: stream/filter not found: %s.%s\n", t, sf.s, sf.f)
			}
		}
		if malformedEntries > 0 {
			logger.Printf(logger.WARN, "%v malformed entries discarded from the DBs\n", malformedEntries)
		}
	}()

	// pattern, stream, fitler → last flush
	flushes := make(map[*PSF]time.Time)
	for {
		var entry LogEntry
		var filter *Filter
		// decode entry
		err := flushDec.Decode(&entry)
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
		flushes[&PSF{entry.Pattern, entry.Stream, entry.Filter}] = entry.T
	}

	lastTimeCpt := int64(0)
	now := time.Now()
	for {
		var entry LogEntry
		var filter *Filter

		// decode entry
		err := logDec.Decode(&entry)
		if err != nil {
			if err == io.EOF {
				break
			}
			malformedEntries++
			continue
		}

		// retrieve related stream & filter
		if entry.Stream == "" && entry.Filter == "" {
			sf, ok := readSF2int[entry.SF]
			if !ok {
				discardedEntries[SF{"", ""}]++
				continue
			}
			entry.Stream = sf.s
			entry.Filter = sf.f
		}
		if stream := c.Streams[entry.Stream]; stream != nil {
			filter = stream.Filters[entry.Filter]
		}
		if filter == nil {
			discardedEntries[SF{entry.Stream, entry.Filter}]++
			continue
		}
		if entry.SF != 0 {
			readSF2int[entry.SF] = SF{entry.Stream, entry.Filter}
		}

		// check if number of patterns is in sync
		if len(entry.Pattern.Split()) != len(filter.pattern) {
			continue
		}

		// check if it hasn't been flushed
		lastGlobalFlush := flushes[&PSF{entry.Pattern, "", ""}].Unix()
		lastLocalFlush := flushes[&PSF{entry.Pattern, entry.Stream, entry.Filter}].Unix()
		entryTime := entry.T.Unix()
		if lastLocalFlush > entryTime || lastGlobalFlush > entryTime {
			continue
		}

		// restore time
		if entry.T.IsZero() {
			entry.T = time.Unix(entry.S, lastTimeCpt)
		}
		lastTimeCpt++

		// store matches
		if !entry.Exec && entry.T.Add(filter.retryDuration).Unix() > now.Unix() {
			if startup {
				startupMatchesC <- PFT{entry.Pattern, filter, entry.T}
			}

			encodeOrFatal(logEnc, entry, writeSF2int, &writeCounter)
		}

		// replay executions
		if entry.Exec && entry.T.Add(*filter.longuestActionDuration).Unix() > now.Unix() {
			if startup {
				flushToMatchesC <- FlushMatchOrder{entry.Pattern, nil}
				filter.sendActions(entry.Pattern, entry.T)
			}

			encodeOrFatal(logEnc, entry, writeSF2int, &writeCounter)
		}
	}
}

func encodeOrFatal(enc *gob.Encoder, entry LogEntry, writeSF2int map[SF]int, writeCounter *int) {
	// Stream/Filter reduction
	sf, ok := writeSF2int[SF{entry.Stream, entry.Filter}]
	if ok {
		entry.SF = sf
		entry.Stream = ""
		entry.Filter = ""
	} else {
		entry.SF = *writeCounter
		writeSF2int[SF{entry.Stream, entry.Filter}] = *writeCounter
		*writeCounter++
	}
	// Time reduction
	if !entry.T.IsZero() {
		entry.S = entry.T.Unix()
		entry.T = time.Time{}
	}
	err := enc.Encode(entry)
	if err != nil {
		logger.Fatalln("Failed to write to new DB:", err)
	}
}
