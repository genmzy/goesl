package goesl

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
)

var logger = &defaultLogger{
	stdlog: log.New(os.Stdout, "<goesl>", log.LstdFlags|log.Lshortfile|log.Lmicroseconds|log.Lmsgprefix),
	level:  LevelInfo,
}

type defaultLogger struct {
	stdlog *log.Logger
	level  Level
}

func (l *defaultLogger) Debugf(format string, v ...interface{}) {
	l.logf(LevelDebug, format, v...)
}

func (l *defaultLogger) Infof(format string, v ...interface{}) {
	l.logf(LevelInfo, format, v...)
}

func (l *defaultLogger) Noticef(format string, v ...interface{}) {
	l.logf(LevelNotice, format, v...)
}

func (l *defaultLogger) Warnf(format string, v ...interface{}) {
	l.logf(LevelWarn, format, v...)
}

func (l *defaultLogger) Errorf(format string, v ...interface{}) {
	l.logf(LevelError, format, v...)
}

func (l *defaultLogger) Fatalf(format string, v ...interface{}) {
	l.logf(LevelFatal, format, v...)
}

func (l *defaultLogger) SetLevel(lv Level) {
	l.level = lv
}

func (l *defaultLogger) SetOutput(w io.Writer) {
	l.stdlog.SetOutput(w)
}

func (l *defaultLogger) SetPrefix(s string) {
	l.stdlog.SetPrefix(s)
}

func (l *defaultLogger) logf(lv Level, format string, v ...interface{}) {
	if l.level > lv {
		return
	}
	buf := bytes.NewBufferString(lv.String())
	buf.WriteString(fmt.Sprintf(format, v...))

	l.stdlog.Output(4, buf.String())
	if lv == LevelFatal {
		os.Exit(1)
	}
}
