package app

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"regexp"
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

func subCommandParse(f *flag.FlagSet, maxRemainingArgs int) {
	help := addBoolFlag([]string{"h", "help"}, f)
	f.Parse(os.Args[2:])
	if *help {
		basicUsage()
		os.Exit(0)
	}
	if len(f.Args()) > maxRemainingArgs {
		fmt.Printf("ERROR unrecognized argument(s): %v\n", f.Args()[maxRemainingArgs:])
		basicUsage()
		os.Exit(1)
	}
}

// FIXME add this options for show & flush
// -l/--limit .STREAM[.FILTER]      # limit to stream and filter
func basicUsage() {
	const (
		bold  = "\033[1m"
		reset = "\033[0m"
	)
	fmt.Print(`usage:

` + bold + `reaction start` + reset + `
  # start the daemon

  # options:
    -c/--config CONFIG_FILE          # configuration file in json, jsonnet or yaml format (required)
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

` + bold + `reaction flush` + reset + ` TARGET
  # run currently active matches and pending actions for the specified TARGET
  # (then show flushed matches and actions)

  # options:
    -s/--socket SOCKET               # path to the client-daemon communication socket
    -f/--format yaml|json            # (default: yaml)

` + bold + `reaction test-regex` + reset + ` REGEX LINE       # test REGEX against LINE
cat FILE | ` + bold + `reaction test-regex` + reset + ` REGEX # test REGEX against each line of FILE
`)
}

//go:embed reaction.yml
var exampleConf string

func Main() {
	if len(os.Args) <= 1 {
		fmt.Println("No argument provided")
		basicUsage()
		os.Exit(1)
	} else if os.Args[1] == "-h" || os.Args[1] == "--help" {
		basicUsage()
		os.Exit(0)
	}
	f := flag.NewFlagSet(os.Args[1], flag.ContinueOnError)
	switch os.Args[1] {
	case "help", "-h", "--help":
		basicUsage()

	case "example-conf":
		subCommandParse(f, 0)
		fmt.Print(exampleConf)

	case "start":
		SocketPath = addSocketFlag(f)
		confFilename := addConfFlag(f)
		subCommandParse(f, 0)
		if *confFilename == "" {
			fmt.Println("no configuration file provided")
			basicUsage()
			os.Exit(1)
		}
		Daemon(*confFilename)

	case "show":
		SocketPath = addSocketFlag(f)
		queryFormat := addFormatFlag(f)
		limit := addLimitFlag(f)
		subCommandParse(f, 0)
		if *queryFormat != "yaml" && *queryFormat != "json" {
			fmt.Println("only yaml and json formats are supported")
			f.PrintDefaults()
			os.Exit(1)
		}
		if *limit != "" {
			fmt.Println("for now, -l/--limit is not supported")
			os.Exit(1)
		}
		// f.Arg(0) is "" if there is no remaining argument
		ClientShow(*limit, *queryFormat)

	case "flush":
		SocketPath = addSocketFlag(f)
		queryFormat := addFormatFlag(f)
		limit := addLimitFlag(f)
		subCommandParse(f, 1)
		if *queryFormat != "yaml" && *queryFormat != "json" {
			fmt.Println("only yaml and json formats are supported")
			f.PrintDefaults()
			os.Exit(1)
		}
		if f.Arg(0) == "" {
			fmt.Println("subcommand flush takes one TARGET argument")
			basicUsage()
			os.Exit(1)
		}
		if *limit != "" {
			fmt.Println("for now, -l/--limit is not supported")
			os.Exit(1)
		}
		ClientFlush(f.Arg(0), *limit, *queryFormat)

	case "test-regex":
		// socket not needed, no interaction with the daemon
		subCommandParse(f, 2)
		if f.Arg(0) == "" {
			fmt.Println("subcommand test-regex takes at least one REGEX argument")
			basicUsage()
			os.Exit(1)
		}
		regex, err := regexp.Compile(f.Arg(0))
		if err != nil {
			fmt.Printf("ERROR the specified regex is invalid: %v", err)
			os.Exit(1)
		}
		if f.Arg(1) == "" {
			fmt.Println("INFO  no second argument: reading from stdin")

			MatchStdin(regex)
		} else {
			Match(regex, f.Arg(1))
		}

	default:
		fmt.Println("subcommand not recognized")
		basicUsage()
		os.Exit(1)
	}
}
