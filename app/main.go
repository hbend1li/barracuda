package app

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"framagit.org/ppom/reaction/logger"
)

func addStringFlag(names []string, defvalue string, f *flag.FlagSet) *string {
	var value string
	for _, name := range names {
		f.StringVar(&value, name, defvalue, "")
	}
	return &value
}

func addBoolFlag(names []string, f *flag.FlagSet) *bool {
	var value bool
	for _, name := range names {
		f.BoolVar(&value, name, false, "")
	}
	return &value
}

var SocketPath *string

func addSocketFlag(f *flag.FlagSet) *string {
	return addStringFlag([]string{"s", "socket"}, "/run/reaction/reaction.sock", f)
}

func addConfFlag(f *flag.FlagSet) *string {
	return addStringFlag([]string{"c", "config"}, "", f)
}

func addFormatFlag(f *flag.FlagSet) *string {
	return addStringFlag([]string{"f", "format"}, "yaml", f)
}

func addLimitFlag(f *flag.FlagSet) *string {
	return addStringFlag([]string{"l", "limit"}, "", f)
}

func addLevelFlag(f *flag.FlagSet) *string {
	return addStringFlag([]string{"l", "loglevel"}, "INFO", f)
}

func addPatternFlag(f *flag.FlagSet) *string {
	return addStringFlag([]string{"p", "pattern"}, "", f)
}

func subCommandParse(f *flag.FlagSet, maxRemainingArgs int) {
	help := addBoolFlag([]string{"h", "help"}, f)
	f.Parse(os.Args[2:])
	if *help {
		basicUsage()
		os.Exit(0)
	}
	// -1 = no limit to remaining args
	if maxRemainingArgs > -1 && len(f.Args()) > maxRemainingArgs {
		fmt.Printf("ERROR unrecognized argument(s): %v\n", f.Args()[maxRemainingArgs:])
		basicUsage()
		os.Exit(1)
	}
}

func basicUsage() {
	const (
		bold  = "\033[1m"
		reset = "\033[0m"
	)
	fmt.Print(
		bold + `reaction help` + reset + `
  # print this help message

` + bold + `reaction start` + reset + `
  # start the daemon

  # options:
    -c/--config CONFIG_FILE          # configuration file in json, jsonnet or yaml format (required)
    -l/--loglevel LEVEL              # minimum log level to show, in DEBUG, INFO, WARN, ERROR, FATAL
                                     # (default: INFO)
    -s/--socket SOCKET               # path to the client-daemon communication socket
                                     # (default: /run/reaction/reaction.sock)

` + bold + `reaction example-conf` + reset + `
  # print a configuration file example

` + bold + `reaction show` + reset + `
  # show current matches and which actions are still to be run
  # (e.g know what is currenly banned)

  # options:
    -s/--socket SOCKET               # path to the client-daemon communication socket
    -f/--format yaml|json            # (default: yaml)
    -l/--limit STREAM[.FILTER]       # only show items related to this STREAM (or STREAM.FILTER)
    -p/--pattern PATTERN             # only show items matching the PATTERN regex

` + bold + `reaction flush` + reset + ` TARGET
  # remove currently active matches and run currently pending actions for the specified TARGET
  # (then show flushed matches and actions)
  # e.g. reaction flush 192.168.1.1 root

  # options:
    -s/--socket SOCKET               # path to the client-daemon communication socket
    -f/--format yaml|json            # (default: yaml)

` + bold + `reaction test-regex` + reset + ` REGEX LINE       # test REGEX against LINE
cat FILE | ` + bold + `reaction test-regex` + reset + ` REGEX # test REGEX against each line of FILE

  # options:
    -c/--config CONFIG_FILE          # configuration file in json, jsonnet or yaml format
                                     # optional: permits to use patterns like <ip> in regex

` + bold + `reaction version` + reset + `
  # print version information

see usage examples, service configurations and good practices
on the ` + bold + `wiki` + reset + `: https://reaction.ppom.me
`)
}

//go:embed example.yml
var exampleConf string

func Main(version, commit string) {
	if len(os.Args) <= 1 {
		logger.Fatalln("No argument provided. Try `reaction help`")
		basicUsage()
		os.Exit(1)
	}
	f := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	switch os.Args[1] {
	case "help", "-h", "-help", "--help":
		basicUsage()

	case "version", "-v", "--version":
		fmt.Printf("reaction version %v commit %v\n", version, commit)

	case "example-conf":
		subCommandParse(f, 0)
		fmt.Print(exampleConf)

	case "start":
		SocketPath = addSocketFlag(f)
		confFilename := addConfFlag(f)
		logLevel := addLevelFlag(f)
		subCommandParse(f, 0)
		if *confFilename == "" {
			logger.Fatalln("no configuration file provided")
			basicUsage()
			os.Exit(1)
		}
		logLevelType := logger.FromString(*logLevel)
		if logLevelType == logger.UNKNOWN {
			logger.Fatalf("Log Level %v not recognized", logLevel)
			basicUsage()
			os.Exit(1)
		}
		logger.SetLogLevel(logLevelType)
		Daemon(*confFilename)

	case "show":
		SocketPath = addSocketFlag(f)
		queryFormat := addFormatFlag(f)
		limit := addLimitFlag(f)
		pattern := addPatternFlag(f)
		subCommandParse(f, 0)
		if *queryFormat != "yaml" && *queryFormat != "json" {
			logger.Fatalln("only yaml and json formats are supported")
			f.PrintDefaults()
			os.Exit(1)
		}
		stream, filter := "", ""
		if *limit != "" {
			splitSF := strings.Split(*limit, ".")
			stream = splitSF[0]
			if len(splitSF) == 2 {
				filter = splitSF[1]
			} else if len(splitSF) > 2 {
				logger.Fatalln("-l/--limit: only one . separator is supported")
			}
		}
		var regex *regexp.Regexp
		var err error
		if *pattern != "" {
			regex, err = regexp.Compile(*pattern)
			if err != nil {
				logger.Fatalln("-p/--pattern: ", err)
			}
		}
		ClientShow(*queryFormat, stream, filter, regex)

	case "flush":
		SocketPath = addSocketFlag(f)
		queryFormat := addFormatFlag(f)
		limit := addLimitFlag(f)
		subCommandParse(f, -1)
		if *queryFormat != "yaml" && *queryFormat != "json" {
			logger.Fatalln("only yaml and json formats are supported")
			f.PrintDefaults()
			os.Exit(1)
		}
		if f.Arg(0) == "" {
			logger.Fatalln("subcommand flush takes one TARGET argument")
			basicUsage()
			os.Exit(1)
		}
		if *limit != "" {
			logger.Fatalln("for now, -l/--limit is not supported")
			os.Exit(1)
		}
		ClientFlush(f.Args(), *limit, *queryFormat)

	case "test-regex":
		// socket not needed, no interaction with the daemon
		confFilename := addConfFlag(f)
		subCommandParse(f, 2)
		if *confFilename == "" {
			logger.Println(logger.WARN, "no configuration file provided. Can't make use of registered patterns.")
		}
		if f.Arg(0) == "" {
			logger.Fatalln("subcommand test-regex takes at least one REGEX argument")
			basicUsage()
			os.Exit(1)
		}
		Match(*confFilename, f.Arg(0), f.Arg(1))

	default:
		logger.Fatalf("subcommand %v not recognized. Try `reaction help`", os.Args[1])
		basicUsage()
		os.Exit(1)
	}
}
