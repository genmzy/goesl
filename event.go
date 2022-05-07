// Copyright 2022 genmzy. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goesl

import (
	"bufio"
	"bytes"
	"fmt"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type mimeMap struct {
	Map       textproto.MIMEHeader
	IsEscaped bool
}

func (m mimeMap) get(key string) string {
	val := m.Map.Get(key)
	if m.IsEscaped {
		val, _ = url.QueryUnescape(val)
	}
	return val
}

func (m mimeMap) String() string {
	var s string
	for k, v := range m.Map {
		val := strings.Join(v, ",")
		if m.IsEscaped {
			val, _ = url.QueryUnescape(val)
		}
		s += fmt.Sprintf("%s: %s\n", k, val)
	}
	return s[:len(s)-1] // remove final newline
}

type FireTime uint64

func (t FireTime) StdTime() time.Time {
	return time.UnixMicro(int64(t))
}

type Event struct {
	UId     string
	Name    EventName
	App     string
	AppData string
	Fire    FireTime
	Type    EventType

	header  mimeMap
	body    mimeMap
	rawBody []byte
}

type EventType int

const (
	EventInvalid EventType = iota
	EventState
	EventConnect
	EventAuth
	EventCommandReply
	EventApiResponse
	EventDisconnect
	EventGeneric
)

//go:generate enumer -type=EventName
type EventName int

const (
	CUSTOM EventName = iota
	CLONE
	CHANNEL_CREATE
	CHANNEL_DESTROY
	CHANNEL_STATE
	CHANNEL_CALLSTATE
	CHANNEL_ANSWER
	CHANNEL_HANGUP
	CHANNEL_HANGUP_COMPLETE
	CHANNEL_EXECUTE
	CHANNEL_EXECUTE_COMPLETE
	CHANNEL_HOLD
	CHANNEL_UNHOLD
	CHANNEL_BRIDGE
	CHANNEL_UNBRIDGE
	CHANNEL_PROGRESS
	CHANNEL_PROGRESS_MEDIA
	CHANNEL_OUTGOING
	CHANNEL_PARK
	CHANNEL_UNPARK
	CHANNEL_APPLICATION
	CHANNEL_ORIGINATE
	CHANNEL_UUID
	API
	LOG
	INBOUND_CHAN
	OUTBOUND_CHAN
	STARTUP
	SHUTDOWN
	PUBLISH
	UNPUBLISH
	TALK
	NOTALK
	SESSION_CRASH
	MODULE_LOAD
	MODULE_UNLOAD
	DTMF
	MESSAGE
	PRESENCE_IN
	NOTIFY_IN
	PRESENCE_OUT
	PRESENCE_PROBE
	MESSAGE_WAITING
	MESSAGE_QUERY
	ROSTER
	CODEC
	BACKGROUND_JOB
	DETECTED_SPEECH
	DETECTED_TONE
	PRIVATE_COMMAND
	HEARTBEAT
	TRAP
	ADD_SCHEDULE
	DEL_SCHEDULE
	EXE_SCHEDULE
	RE_SCHEDULE
	RELOADXML
	NOTIFY
	PHONE_FEATURE
	PHONE_FEATURE_SUBSCRIBE
	SEND_MESSAGE
	RECV_MESSAGE
	REQUEST_PARAMS
	CHANNEL_DATA
	GENERAL
	COMMAND
	SESSION_HEARTBEAT
	CLIENT_DISCONNECTED
	SERVER_DISCONNECTED
	SEND_INFO
	RECV_INFO
	RECV_RTCP_MESSAGE
	CALL_SECURE
	NAT
	RECORD_START
	RECORD_STOP
	PLAYBACK_START
	PLAYBACK_STOP
	CALL_UPDATE
	FAILURE
	SOCKET_DATA
	MEDIA_BUG_START
	MEDIA_BUG_STOP
	CONFERENCE_DATA_QUERY
	CONFERENCE_DATA
	CALL_SETUP_REQ
	CALL_SETUP_RESULT
	CALL_DETAIL
	DEVICE_STATE
	ALL
)

func (e Event) GetTextBody() string {
	if e.Type == EventApiResponse {
		resp := strings.TrimSpace(string(e.rawBody))
		return string(resp)
	}
	slen := e.body.get("Content-Length")
	if slen == "" {
		return ""
	}
	bblen, err := strconv.Atoi(slen)
	if err != nil {
		return ""
	}
	blen := len(e.rawBody)
	return string(e.rawBody[blen-bblen : blen-1])
}

// Get retrieves the value of header from Event header or (if not found) from Event body.
// The value is returned unescaped and is empty if not found anywhere.
func (e Event) Get(header string) string {
	val := e.header.get(header)
	if val == "" {
		val = e.body.get(header)
	}
	return val
}

func (e Event) String() string {
	body, _ := url.QueryUnescape(string(e.rawBody))
	return fmt.Sprintf("%s\n.\n%s====================\n", e.header, body)
}

func (e *Event) parseTextBody() error {
	var err error
	buf := bufio.NewReader(bytes.NewReader(e.rawBody))
	if e.body.Map, err = textproto.NewReader(buf).ReadMIMEHeader(); err != nil {
		return fmt.Errorf("parse text body: %v", err)
	}
	e.body.IsEscaped = true
	e.UId = e.Get("Unique-ID")
	e.Name, _ = EventNameString(e.Get("Event-Name"))
	e.App = e.Get("Application")
	e.AppData = strings.TrimSpace(e.Get("Application-Data"))
	t, err := strconv.ParseUint(e.Get("Event-Date-Timestamp"), 10, 64)
	e.Fire = FireTime(t) // ignore err or not
	return err
}
