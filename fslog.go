// This file convert from freeswitch 1.10.9 switch_log.c
package goesl

import (
	"errors"
	"strconv"
	"strings"
)

type FslogLevel int

var (
	FslogLevel_DEBUG10 FslogLevel = 110  // SWITCH_LOG_DEBUG10
	FslogLevel_DEBUG9  FslogLevel = 109  // SWITCH_LOG_DEBUG9
	FslogLevel_DEBUG8  FslogLevel = 108  // SWITCH_LOG_DEBUG8
	FslogLevel_DEBUG7  FslogLevel = 107  // SWITCH_LOG_DEBUG7
	FslogLevel_DEBUG6  FslogLevel = 106  // SWITCH_LOG_DEBUG6
	FslogLevel_DEBUG5  FslogLevel = 105  // SWITCH_LOG_DEBUG5
	FslogLevel_DEBUG4  FslogLevel = 104  // SWITCH_LOG_DEBUG4
	FslogLevel_DEBUG3  FslogLevel = 103  // SWITCH_LOG_DEBUG3
	FslogLevel_DEBUG2  FslogLevel = 102  // SWITCH_LOG_DEBUG2
	FslogLevel_DEBUG1  FslogLevel = 101  // SWITCH_LOG_DEBUG1
	FslogLevel_DEBUG   FslogLevel = 7    // SWITCH_LOG_DEBUG
	FslogLevel_INFO    FslogLevel = 6    // SWITCH_LOG_INFO
	FslogLevel_NOTICE  FslogLevel = 5    // SWITCH_LOG_NOTICE
	FslogLevel_WARNING FslogLevel = 4    // SWITCH_LOG_WARNING
	FslogLevel_ERROR   FslogLevel = 3    // SWITCH_LOG_ERROR
	FslogLevel_CRIT    FslogLevel = 2    // SWITCH_LOG_CRIT
	FslogLevel_ALERT   FslogLevel = 1    // SWITCH_LOG_ALERT
	FslogLevel_CONSOLE FslogLevel = 0    // SWITCH_LOG_CONSOLE
	FslogLevel_DISABLE FslogLevel = -1   // SWITCH_LOG_DISABLE
	FslogLevel_INVALID FslogLevel = 64   // SWITCH_LOG_INVALID
	FslogLevel_UNINIT  FslogLevel = 1000 // SWITCH_LOG_UNINIT
)

var fslogLevelStrings = [9]string{
	"DISABLE",
	"CONSOLE",
	"ALERT",
	"CRIT",
	"ERR",
	"WARNING",
	"NOTICE",
	"INFO",
	"DEBUG",
}

func FslogString2Level(s string) FslogLevel {
	x := 0
	lv := FslogLevel_INVALID
	if x, err := strconv.Atoi(s); err == nil {
		// string number
		if FslogLevel(x) > FslogLevel_INVALID {
			return FslogLevel(FslogLevel_INVALID - 1)
		} else if x < 0 {
			return 0
		} else {
			return FslogLevel(x)
		}
	}
	for x = 0; ; x++ {
		if x >= len(fslogLevelStrings) {
			break
		}
		if strings.EqualFold(fslogLevelStrings[x], s) {
			lv = FslogLevel(x - 1)
			break
		}
	}
	return lv
}

func (lv FslogLevel) String() string {
	if lv > FslogLevel_DEBUG {
		lv = FslogLevel_DEBUG
	}
	return fslogLevelStrings[lv+1]
}

type Fslog struct {
	Content  string
	File     string
	UserData string
	Func     string
	Line     int
	Level    FslogLevel
}

func Event2Fslog(e Event) (fslog Fslog, err error) {
	if e.Type != EventLog {
		err = errors.New("not a FreeSWITCH log event")
		return
	}
	fslog.Level = FslogString2Level(e.Get("Log-Level"))
	fslog.Content = e.GetTextBody()
	fslog.File = e.Get("Log-File")
	fslog.Func = e.Get("Log-Func")
	fslog.UserData = e.Get("User-Data")
	fslog.Line, err = strconv.Atoi(e.Get("Log-Line"))
	return
}
