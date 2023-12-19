package goesl

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
)

var defaultLogger = &defaultLoggerType{
	stdlog:   log.New(os.Stdout, "\033[1;34m<goesl>\033[0m ", log.LstdFlags|log.Lshortfile|log.Lmicroseconds|log.Lmsgprefix),
	buf:      &bytes.Buffer{},
	level:    LevelInfo,
	terminal: true,
}

type defaultLoggerType struct {
	stdlog   *log.Logger
	buf      *bytes.Buffer
	level    Level
	terminal bool
}

func (l *defaultLoggerType) Debugf(format string, v ...any) {
	l.logf(LevelDebug, format, v...)
}

func (l *defaultLoggerType) Infof(format string, v ...any) {
	l.logf(LevelInfo, format, v...)
}

func (l *defaultLoggerType) Noticef(format string, v ...any) {
	l.logf(LevelNotice, format, v...)
}

func (l *defaultLoggerType) Warnf(format string, v ...any) {
	l.logf(LevelWarn, format, v...)
}

func (l *defaultLoggerType) Errorf(format string, v ...any) {
	l.logf(LevelError, format, v...)
}

func (l *defaultLoggerType) Fatalf(format string, v ...any) {
	l.logf(LevelFatal, format, v...)
}

func (l *defaultLoggerType) setLevel(lv Level) {
	l.level = lv
}

func (l *defaultLoggerType) setOutput(w io.Writer) {
	if w != os.Stderr && w != os.Stdout {
		l.terminal = false
	}
	l.stdlog.SetOutput(w)
}

func (l *defaultLoggerType) setPrefix(s string) {
	if l.terminal {
		l.stdlog.SetPrefix(fmt.Sprintf("\033[1;34m%s\033[0m ", s))
	} else {
		l.stdlog.SetPrefix(fmt.Sprintf("%s ", s))
	}
}

var colorFormats = map[Level]string{
	LevelDebug:  "\033[1;33m%s\033[0m",
	LevelInfo:   "\033[1;32m%s\033[0m",
	LevelNotice: "\033[1;36m%s\033[0m",
	LevelWarn:   "\033[1;35m%s\033[0m",
	LevelError:  "\033[1;31m%s\033[0m",
	LevelFatal:  "\033[1;31m%s\033[0m",
}

func (l *defaultLoggerType) colorLevelStr(lv Level) string {
	if !l.terminal {
		return lv.String()
	}
	return fmt.Sprintf(colorFormats[lv], lv.String())
}

func (l *defaultLoggerType) logf(lv Level, format string, v ...any) {
	if l.level > lv {
		return
	}
	l.buf.Truncate(0)
	l.buf.WriteString(l.colorLevelStr(lv))
	l.buf.WriteByte(' ')
	fmt.Fprintf(l.buf, format, v...)

	l.stdlog.Output(3, l.buf.String())
	if lv == LevelFatal {
		os.Exit(1)
	}
}
