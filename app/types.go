package app

import (
	"encoding/gob"
	"os"
	"regexp"
	"time"
)

type Conf struct {
	Patterns map[string]*Pattern `yaml:"patterns"`
	Streams  map[string]*Stream  `yaml:"streams"`
}

type Pattern struct {
	Regex  string   `yaml:"regex"`
	Ignore []string `yaml:"ignore"`

	name           string `yaml:"-"`
	nameWithBraces string `yaml:"-"`
}

// Stream, Filter & Action structures must never be copied.
// They're always referenced through pointers

type Stream struct {
	name string `yaml:"-"`

	Cmd     []string           `yaml:"cmd"`
	Filters map[string]*Filter `yaml:"filters"`
}

type Filter struct {
	stream *Stream `yaml:"-"`
	name   string  `yaml:"-"`

	Regex         []string        `yaml:"regex"`
	compiledRegex []regexp.Regexp `yaml:"-"`
	pattern       *Pattern        `yaml:"-"`

	Retry         int           `yaml:"retry"`
	RetryPeriod   string        `yaml:"retry-period"`
	retryDuration time.Duration `yaml:"-"`

	Actions                map[string]*Action `yaml:"actions"`
	longuestActionDuration *time.Duration
}

type Action struct {
	filter *Filter `yaml:"-"`
	name   string  `yaml:"-"`

	Cmd []string `yaml:"cmd"`

	After         string        `yaml:"after"`
	afterDuration time.Duration `yaml:"-"`

	OnExit bool `yaml:"onexit"`
}

type LogEntry struct {
	T              time.Time
	Pattern        string
	Stream, Filter string
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

type MatchesMap map[*Filter]map[string][]time.Time

// Helper structs made to carry information across channels
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
