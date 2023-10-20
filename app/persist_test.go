package app

import (
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"
)

func confInit() *Conf {
	m3 := time.Duration(3 * time.Minute)
	pattern := Pattern{"[0-9]{2}", []string{}, "ip", "<ip>"}
	stream0 := Stream{"stream0", []string{}, make(map[string]*Filter)}
	stream1 := Stream{"stream1", []string{}, make(map[string]*Filter)}
	filter0 := Filter{&stream0, "filter0", []string{}, []regexp.Regexp{}, &pattern, 1, "", m3, make(map[string]*Action), &m3}
	filter1 := Filter{&stream1, "filter1", []string{}, []regexp.Regexp{}, &pattern, 2, "", m3, make(map[string]*Action), &m3}
	action0 := Action{&filter0, "action0", []string{}, "", 0, false}
	action1 := Action{&filter1, "action1", []string{}, "", 0, false}
	conf := Conf{
		make(map[string]*Pattern),
		make(map[string]*Stream),
		[][]string{},
		[][]string{},
	}

	conf.Patterns[pattern.name] = &pattern

	filter0.Actions[action0.name] = &action0
	filter1.Actions[action1.name] = &action1

	stream0.Filters[filter0.name] = &filter0
	stream1.Filters[filter1.name] = &filter1

	conf.Streams[stream0.name] = &stream0
	conf.Streams[stream1.name] = &stream1

	return &conf
}

func TestPersistence(t *testing.T) {
	dirname, err := os.MkdirTemp(os.TempDir(), "gotest-")
	if err != nil {
		t.Skip("Couldn't create a temporary directory, so can't run the test")
	}
	err = os.Chdir(dirname)
	if err != nil {
		t.Skip("Couldn't go to the created temporary directory, so can't run the test")
	}

	startupMatchesC = make(chan PFT)
	matchesC = make(chan PFT)
	logsC = make(chan LogEntry)
	actionsC = make(chan PAT)
	flushToMatchesC = make(chan FlushMatchOrder)
	flushToActionsC = make(chan FlushActionOrder)
	flushToDatabaseC = make(chan LogEntry)
	stopActions = make(chan bool)
	stopStreams = make(chan bool)
	actions = make(ActionsMap)
	matches = make(MatchesMap)
	conf := confInit()

	// Send logs
	go DatabaseManager(conf)
	times := make([]time.Time, 10)
	times[0] = time.Now()
	for i := 1; i < 10; i++ {
		times[i] = times[i-1].Add(1 * time.Millisecond)
	}
	entries := []LogEntry{
		{times[0], "01", "stream0", "filter0", 0, true},
		{times[1], "10", "stream1", "filter1", 0, false},
		{times[2], "01", "stream0", "filter0", 0, true},
		{times[3], "10", "stream1", "filter1", 0, true},
		{times[4], "01", "stream0", "filter0", 0, true},
		{times[5], "10", "stream1", "filter1", 0, false},
		// Must be unknown
		{times[6], "10", "stream1", "filter0", 0, false},
		{times[7], "10", "stream0", "filter1", 0, false},
	}
	for _, e := range entries {
		logsC <- e
	}
	t.Run("Logs sent", func(t *testing.T) {})

	// Retrieve logs
	startupMatchesC = make(chan PFT)
	go DatabaseManager(conf)
	assertMatch := func(l LogEntry) {
		t.Run(fmt.Sprintf("Receive %v", l), func(t *testing.T) {
			time.Sleep(time.Second)
			select {
			case <-flushToMatchesC:
				// no-op
			case pat, ok := <-actionsC:
				if !l.Exec {
					t.Error("Was expecting Exec=true but got Exec=false")
				}
				if !ok {
					t.Error("Exec: Was expecting something but channel was closed")
				}
				if pat.p != l.Pattern {
					t.Errorf("Exec: Was expecting pattern %v but got %v", l.Pattern, pat.p)
				}
				if pat.a.filter.name != l.Filter {
					t.Errorf("Match: Was expecting filter name %v but got %v", l.Filter, pat.a.filter)
				}
				if pat.a.filter.stream.name != l.Stream {
					t.Errorf("Match: Was expecting stream name %v but got %v", l.Stream, pat.a.filter.stream.name)
				}
				if pat.t != l.T {
					t.Errorf("Match: Was expecting time %v but got %v", l.T, pat.t)
				}
			case pft, ok := <-startupMatchesC:
				if l.Exec {
					t.Error("Was expecting Exec=false but got Exec=true")
				}
				if !ok {
					t.Error("Exec: Was expecting something but channel was closed")
				}
				fmt.Println("pft", pft.p, pft.f, pft.t)
				if pft.p != l.Pattern {
					t.Errorf("Match: Was expecting pattern %v but got %v", l.Pattern, pft.p)
				}
				if pft.f.name != l.Filter {
					t.Errorf("Match: Was expecting filter name %v but got %v", l.Filter, pft.f)
				}
				if pft.f.stream.name != l.Stream {
					t.Errorf("Match: Was expecting stream name %v but got %v", l.Stream, pft.f.stream.name)
				}
				if pft.t != l.T {
					t.Errorf("Match: Was expecting time %v but got %v", l.T, pft.t)
				}
			}
		})
	}
	for i := 0; i < 6; i++ {
		assertMatch(entries[i])
	}
	t.Run("Test that startupMatchesC is closed", func(t *testing.T) {
		l, ok := <-startupMatchesC
		if ok {
			t.Errorf("Was expecting a closed channel but got %v", l)
		}
	})
}
