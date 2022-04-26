// Copyright 2022 genmzy. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goesl

import (
	"fmt"
	"io"
)

// Logger is a logger interface that output logs with a format.
type Logger interface {
	SetLevel(Level)
	SetOutput(io.Writer)

	Debugf(format string, v ...interface{})
	Infof(format string, v ...interface{})
	Noticef(format string, v ...interface{})
	Warnf(format string, v ...interface{})
	Errorf(format string, v ...interface{})
	Fatalf(format string, v ...interface{})
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
	"[Debug] ",
	"[Info] ",
	"[Notice] ",
	"[Warn] ",
	"[Error] ",
	"[Fatal] ",
}

func (lv Level) String() string {
	if lv >= LevelDebug && lv <= LevelFatal {
		return strs[lv]
	}
	return fmt.Sprintf("[?%d] ", lv)
}
