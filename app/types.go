package app

import (
	"encoding/gob"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

type Conf struct {
	Concurrency int                 `json:"concurrency"`
	Patterns    map[string]*Pattern `json:"patterns"`
	Streams     map[string]*Stream  `json:"streams"`
	Start       [][]string          `json:"start"`
	Stop        [][]string          `json:"stop"`
}

type Pattern struct {
	Regex  string   `json:"regex"`
	Ignore []string `json:"ignore"`

	IgnoreRegex         []string        `json:"ignoreregex"`
	compiledIgnoreRegex []regexp.Regexp `json:"-"`

	Name           string `json:"-"`
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
	Pattern       []*Pattern      `json:"-"`

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
	Pattern        Match
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

// This is a "\x00" Joined string
// which contains all matches on a line.
type Match string

func (m *Match) Split() []string {
	return strings.Split(string(*m), "\x00")
}
func JoinMatch(mm []string) Match {
	return Match(strings.Join(mm, "\x00"))
}
func WithBrackets(mm []string) string {
	var b strings.Builder
	for _, match := range mm {
		fmt.Fprintf(&b, "[%s]", match)
	}
	return b.String()
}

// Helper structs made to carry information
// Stream, Filter
type SF struct{ s, f string }

// Pattern, Stream, Filter
type PSF struct {
	p    Match
	s, f string
}

type PF struct {
	p Match
	f *Filter
}
type PFT struct {
	p Match
	f *Filter
	t time.Time
}
type PA struct {
	p Match
	a *Action
}
type PAT struct {
	p Match
	a *Action
	t time.Time
}

type FlushMatchOrder struct {
	p   Match
	ret chan MatchesMap
}
type FlushActionOrder struct {
	p   Match
	ret chan ActionsMap
}
