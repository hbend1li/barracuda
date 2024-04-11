package cmd

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"framagit.org/ppom/reaction/app"
	"framagit.org/ppom/reaction/logger"
	"github.com/spf13/cobra"
)

// Main

func Execute(version, commit string) {
	rootCmd.Version = fmt.Sprintf("%v commit %v\n", version, commit)
	// Let cobra parse the args and launch reaction
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

// Initialization

var rootCmd = &cobra.Command{
	Use:   "reaction",
	Short: "Scan logs and take action",
	Long: `A daemon that scans program outputs for repeated patterns, and takes action.
Aims at being more versatile and flexible than fail2ban, while being faster and having simpler configuration.

See usage examples, service configurations and good practices
on the wiki: https://reaction.ppom.me`,
}

func init() {
	rootCmd.AddGroup(
		&cobra.Group{ID: "daemon", Title: "Daemon:"},
		&cobra.Group{ID: "client", Title: "Client commands:"},
	)

	// start
	var startCmd = &cobra.Command{
		Use:   "start -c CONFIG_FILE",
		Short: "Start reaction daemon",
		Long: `Start the reaction daemon.
It will create a socket to interact with client commands.`,
		GroupID:    "daemon",
		SuggestFor: []string{"daemon"},
		Args:       cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			logLevelType := logger.FromString(logLevel)
			if logLevelType == logger.UNKNOWN {
				return errors.New(fmt.Sprintf("Log Level %v not recognized", logLevel))
			}
			logger.SetLogLevel(logLevelType)
			app.Daemon(confPath)
			return nil
		},
	}
	rootCmd.AddCommand(startCmd)
	addLogLevelFlag(startCmd)
	addSocketFlag(startCmd)
	addConfFlag(startCmd)
	startCmd.MarkFlagRequired("config")

	// show
	var showCmd = &cobra.Command{
		Use:     "show [flags]",
		Short:   "Show current matches and actions",
		Long:    "Show current matches and which actions are still to be run (e.g. know what is currently banned)",
		GroupID: "client",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "yaml" && format != "json" {
				return errors.New("only yaml and json formats are supported")
			}
			stream, filter := "", ""
			if limit != "" {
				splitSF := strings.Split(limit, ".")
				if len(splitSF) > 2 {
					return errors.New("-l/--limit: only one . separator is supported")
				}
				stream = splitSF[0]
				if len(splitSF) == 2 {
					filter = splitSF[1]
				}
			}
			var regex *regexp.Regexp
			var err error
			if pattern != "" {
				regex, err = regexp.Compile(pattern)
				if err != nil {
					return err //ors.New(fmt.Sprintf("-p/--pattern: %v", err))
				}
			}
			app.ClientShow(format, stream, filter, regex)
			return nil
		},
	}
	rootCmd.AddCommand(showCmd)
	addSocketFlag(showCmd)
	addFormatFlag(showCmd)
	addLimitFlag(showCmd)
	addPatternFlag(showCmd)

	// flush
	var flushCmd = &cobra.Command{
		Use:   "flush [flags] TARGET",
		Short: "Remove a target from reaction (e.g. unban)",
		Long: `Remove currently active matches and run currently pending actions for the specified TARGET. (e.g. unban)
Then prints the flushed matches and actions`,
		GroupID:    "client",
		Args:       cobra.MinimumNArgs(1),
		SuggestFor: []string{"unban", "remove", "forget"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "yaml" && format != "json" {
				return errors.New("only yaml and json formats are supported")
			}
			// if limit != "" {
			// 	return errors.New("for now, -l/--limit is not supported")
			// }
			app.ClientFlush(args[0], limit, format)
			return nil
		},
	}
	rootCmd.AddCommand(flushCmd)
	addSocketFlag(flushCmd)
	addFormatFlag(flushCmd)
	// addLimitFlag(showCmd)

	// test-regex
	var testRegexCmd = &cobra.Command{
		Use: `test-regex [flags] REGEX [LINE]
  cat FILE | reaction test-regex [flags] REGEX
  cat      | reaction test-regex [flags] REGEX # keyboard interactive`,
		Short: "Test a regex",
		Long: `Test a REGEX against one LINE, or against standard input.
Giving a configuration file permits to use its patterns in REGEX.`,
		Args: cobra.RangeArgs(1, 2),
		Run: func(cmd *cobra.Command, args []string) {
			if confPath == "" {
				logger.Println(logger.WARN, "no configuration file provided. Can't make use of registered patterns.")
			}
			var arg0, arg1 string
			arg0 = args[0]
			if len(args) == 2 {
				arg1 = args[1]
			}
			app.Match(confPath, arg0, arg1)
		},
	}
	rootCmd.AddCommand(testRegexCmd)
	addConfFlag(testRegexCmd)
}

// Flags

var confPath, logLevel, format, limit, pattern string

func addSocketFlag(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&app.SocketPath,
		"socket", "s",
		"/run/reaction/reaction.sock",
		"path to the client-daemon communication socket",
	)
}

func addConfFlag(cmd *cobra.Command) {
	var required string
	if strings.HasPrefix(cmd.Use, "start") {
		required = " (required)"
	}
	cmd.Flags().StringVarP(&confPath,
		"config", "c",
		"",
		"configuration file in json, jsonnet or yaml format"+required,
	)
	cmd.RegisterFlagCompletionFunc("loglevel", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"/etc/reaction.jsonnet", "/etc/reaction.yml", "~/.config/reaction.jsonnet", "~/.config/reaction.yml"}, 0
	})
}

func addFormatFlag(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&format,
		"format", "f",
		"yaml",
		`how to format output: "json" or "yaml"`,
	)
	cmd.RegisterFlagCompletionFunc("loglevel", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"yaml", "json"}, 0
	})
}

func addLimitFlag(cmd *cobra.Command) {
	cmdStr := cmd.Use[0:strings.Index(cmd.Use, " ")]
	cmd.Flags().StringVarP(&limit,
		"limit", "l",
		"",
		fmt.Sprintf(`only %s items related to this STREAM[.FILTER]`, cmdStr),
	)
}

func addPatternFlag(cmd *cobra.Command) {
	cmdStr := cmd.Use[0:strings.Index(cmd.Use, " ")]
	cmd.Flags().StringVarP(&pattern,
		"pattern", "p",
		"",
		fmt.Sprintf("only %v items matching PATTERN regex", cmdStr),
	)
}

func addLogLevelFlag(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&logLevel,
		"loglevel", "l",
		"INFO",
		"minimum log level to show, in DEBUG, INFO, WARN, ERROR, FATAL",
	)
	cmd.RegisterFlagCompletionFunc("loglevel", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}, 0
	})
}
