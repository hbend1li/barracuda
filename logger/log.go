package logger

import "log"

type Level int

const (
	DEBUG = Level(1)
	INFO  = Level(2)
	WARN  = Level(3)
	ERROR = Level(4)
	FATAL = Level(5)
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

var LogLevel Level = 2

func SetLogLevel(level Level) {
	LogLevel = level
}

func Println(level Level, args ...any) {
	if level >= LogLevel {
		log.Println(level, args)
	}
}

func Printf(level Level, format string, args ...any) {
	if level >= LogLevel {
		log.Printf(level.String()+format, args)
	}
}

func Fatalln(args ...any) {
	level := FATAL
	log.Fatalln(level.String(), args)
}

func Fatalf(format string, args ...any) {
	level := FATAL
	log.Fatalf(level.String()+format, args)
}
