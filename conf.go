package main

import (
	// "flag"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type Conf struct {
	// Definitions []string
	Streams     []struct {
		Cmd     string
		Filters []struct {
			Regex       []string
			Retry       uint
			RetryPeriod string `yaml:"retry-period"`
			Actions     []struct {
				Cmd   string
				After string `yaml:",omitempty"`
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
	log.Println(conf)

	yaml, err := yaml.Marshal(conf)
	if err != nil {
		log.Fatalln("Failed to rewrite configuration file:", err)
	}
	log.Println(string(yaml))
	return &conf
}

func parseArgs() map[string]string {
	var args map[string]string
	return args
}
