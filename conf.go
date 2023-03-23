package main

import (
	// "flag"
	"fmt"
	"log"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

type Conf struct {
	Streams map[string]Stream
}

type Stream struct {
	Cmd     []string
	Filters map[string]*Filter
}

type Filter struct {
	Regex         []string
	compiledRegex []regexp.Regexp
	Retry         uint
	RetryPeriod   string `yaml:"retry-period"`
	Actions       map[string]*Action
}

type Action struct {
	name, filterName, streamName string
	Cmd                          []string
	After                        string `yaml:",omitempty"`
}

func (c *Conf) setup() {
	for streamName, stream := range c.Streams {
		for filterName, filter := range stream.Filters {
			// Compute Regexes
			for _, regex := range filter.Regex {
				filter.compiledRegex = append(filter.compiledRegex, *regexp.MustCompile(regex))
			}
			// Give all relevant infos to Actions
			for actionName, action := range filter.Actions {
				action.name = actionName
				action.filterName = filterName
				action.streamName = streamName
			}
		}
	}
}

func parseConf(filename string) *Conf {

	data, err := os.ReadFile(filename)

	if err != nil {
		log.Fatalln("Failed to read configuration file:", err)
	}

	var conf Conf
	err = yaml.Unmarshal(data, &conf)
	if err != nil {
		log.Fatalln("Failed to parse configuration file:", err)
	}

	conf.setup()
	fmt.Printf("conf.Streams[0].Filters[0].Actions: %s\n", conf.Streams["tailDown"].Filters["lookForProuts"].Actions)

	return &conf
}

func parseArgs() map[string]string {
	var args map[string]string
	return args
}
