package logger

import "log"

type Level int

const (
	UNKNOWN = Level(-1)
	DEBUG   = Level(1)
	INFO    = Level(2)
	WARN    = Level(3)
	ERROR   = Level(4)
	FATAL   = Level(5)
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG "
	case INFO:
		return "INFO  "
	case WARN:
		return "WARN  "
	case ERROR:
		return "ERROR "
	case FATAL:
		return "FATAL "
	default:
		return "????? "
	}
}

func FromString(s string) Level {
	switch s {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN":
		return WARN
	case "ERROR":
		return ERROR
	case "FATAL":
		return FATAL
	default:
		return UNKNOWN
	}
}

var LogLevel Level = 2

func SetLogLevel(level Level) {
	LogLevel = level
}

func Println(level Level, args ...any) {
	if level >= LogLevel {
		newargs := make([]any, 0)
		newargs = append(newargs, level)
		newargs = append(newargs, args...)
		log.Println(newargs...)
	}
}

func Printf(level Level, format string, args ...any) {
	if level >= LogLevel {
		log.Printf(level.String()+format, args...)
	}
}

func Fatalln(args ...any) {
	newargs := make([]any, 0)
	newargs = append(newargs, FATAL)
	newargs = append(newargs, args...)
	log.Fatalln(newargs...)
}

func Fatalf(format string, args ...any) {
	level := FATAL
	log.Fatalf(level.String()+format, args)
}
