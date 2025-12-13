package logger

import (
	"fmt"
	"os"
	"time"
)

// Log levels
const (
	INFO  = "INFO"
	ERROR = "ERROR"
	WARN  = "WARN"
	DEBUG = "DEBUG"
)

func log(level, format string, args ...interface{}) {
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000000Z")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s %s\n", timestamp, level, msg)
}

func Info(format string, args ...interface{}) {
	log(INFO, format, args...)
}

func Error(format string, args ...interface{}) {
	log(ERROR, format, args...)
}

func Warn(format string, args ...interface{}) {
	log(WARN, format, args...)
}

func Debug(format string, args ...interface{}) {
	log(DEBUG, format, args...)
}
