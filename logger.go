package goesl

import "fmt"

// Logger is a logger interface that output logs with a format.
type Logger interface {
	Debugf(format string, v ...any)
	Infof(format string, v ...any)
	Noticef(format string, v ...any)
	Warnf(format string, v ...any)
	Errorf(format string, v ...any)
	Fatalf(format string, v ...any)
}

// Level defines the priority of a log message.
// When a logger is configured with a level, any log message with a lower
// log level (smaller by integer comparison) will not be output.
type Level int

// The levels of logs.
const (
	LevelDebug Level = iota
	LevelInfo
	LevelNotice
	LevelWarn
	LevelError
	LevelFatal
)

var strs = []string{
	"[Debug]",
	"[Info]",
	"[Notice]",
	"[Warn]",
	"[Error]",
	"[Fatal]",
}

func (lv Level) String() string {
	if lv >= LevelDebug && lv <= LevelFatal {
		return strs[lv]
	}
	return fmt.Sprintf("[?%d] ", lv)
}

func Debugf(format string, v ...any) {
	defaultLogger.logf(LevelDebug, format, v...)
}

func Infof(format string, v ...any) {
	defaultLogger.logf(LevelInfo, format, v...)
}

func Noticef(format string, v ...any) {
	defaultLogger.logf(LevelNotice, format, v...)
}

func Warnf(format string, v ...any) {
	defaultLogger.logf(LevelWarn, format, v...)
}

func Errorf(format string, v ...any) {
	defaultLogger.logf(LevelError, format, v...)
}

func Fatalf(format string, v ...any) {
	defaultLogger.logf(LevelFatal, format, v...)
}
