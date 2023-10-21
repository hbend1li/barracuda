package app

import (
	"encoding/gob"
	"os"
	"regexp"
	"time"
)

type Conf struct {
	Patterns map[string]*Pattern `json:"patterns"`
	Streams  map[string]*Stream  `json:"streams"`
	Start    [][]string          `json:"start"`
	Stop     [][]string          `json:"stop"`
}

type Pattern struct {
	Regex  string   `json:"regex"`
	Ignore []string `json:"ignore"`

	name           string `json:"-"`
	nameWithBraces string `json:"-"`
}

// Stream, Filter & Action structures must never be copied.
// They're always referenced through pointers

type Stream struct {
	name string `json:"-"`

	Cmd     []string           `json:"cmd"`
	Filters map[string]*Filter `json:"filters"`
}

type Filter struct {
	stream *Stream `json:"-"`
	name   string  `json:"-"`

	Regex         []string        `json:"regex"`
	compiledRegex []regexp.Regexp `json:"-"`
	pattern       *Pattern        `json:"-"`

	Retry         int           `json:"retry"`
	RetryPeriod   string        `json:"retryperiod"`
	retryDuration time.Duration `json:"-"`

	Actions                map[string]*Action `json:"actions"`
	longuestActionDuration *time.Duration
}

type Action struct {
	filter *Filter `json:"-"`
	name   string  `json:"-"`

	Cmd []string `json:"cmd"`

	After         string        `json:"after"`
	afterDuration time.Duration `json:"-"`

	OnExit bool `json:"onexit"`
}

type LogEntry struct {
	T              time.Time
	S              int64
	Pattern        string
	Stream, Filter string
	SF             int
	Exec           bool
}

type ReadDB struct {
	file *os.File
	dec  *gob.Decoder
}

type WriteDB struct {
	file *os.File
	enc  *gob.Encoder
}

type MatchesMap map[PF]map[time.Time]struct{}
type ActionsMap map[PA]map[time.Time]struct{}

// Helper structs made to carry information
type SF struct{ s, f string }
type PSF struct{ p, s, f string }
type PF struct {
	p string
	f *Filter
}
type PFT struct {
	p string
	f *Filter
	t time.Time
}
type PA struct {
	p string
	a *Action
}
type PAT struct {
	p string
	a *Action
	t time.Time
}

type FlushMatchOrder struct {
	p   string
	ret chan MatchesMap
}
type FlushActionOrder struct {
	p   string
	ret chan ActionsMap
}
