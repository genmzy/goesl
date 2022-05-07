// Copyright 2022 genmzy. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goesl

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type mimeMap struct {
	headers textproto.MIMEHeader
	escaped bool
}

var ErrMismatchEventType = errors.New("mismatch event type")

func (m mimeMap) get(key string) string {
	val := m.headers.Get(key)
	if m.escaped {
		val, _ = url.QueryUnescape(val)
	}
	return val
}

func (m mimeMap) String() string {
	var s string
	for k, v := range m.headers {
		val := strings.Join(v, ",")
		if m.escaped {
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
	Type EventType

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
		Warnf("content length parse error: %v", err)
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
	if e.body.headers, err = textproto.NewReader(buf).ReadMIMEHeader(); err != nil {
		return fmt.Errorf("parse text body: %v", err)
	}
	e.body.escaped = true // all generic events are regard as escaped
	return err
}

// --------------------- helper to event get -------------------------- //

// generic event headers
const (
	// Core uuid
	Core_Uuid = "Core-UUID"
	// event innate attributions
	Event_Name           = "Event-Name"
	Event_Sequence       = "Event-Sequence"
	FreeSWITCH_IPv4      = "FreeSWITCH-IPv4"
	Event_Date_Timestamp = "Event-Date-Timestamp"
	// background job related
	BgJob_Uuid        = "Job-Uuid"
	BgJob_Command     = "Job-Command"
	BgJob_Command_Arg = "Job-Command-Arg"
	// leg uuid
	Unique_ID           = "Unique-ID"
	Channel_State       = "Channel-State"
	Call_Direction      = "Call-Direction"
	Other_Leg_Unique_ID = "Other-Leg-Unique-ID"
	// caller
	Caller_ID_Number    = "Caller-Caller-ID-Number"
	Callee_ID_Number    = "Caller-Destination-Number"
	Caller_Network_Addr = "Caller-Network-Addr"
	// variables
	Sip_From_Uri = "variable_sip_from_uri"
	// digits
	DTMF_Digit  = "DTMF-Digit"
	DTMF_Source = "DTMF-Source"
	// hangup cause
	Hangup_Cause = "Hangup-Cause"
	// current application
	CurrentApp     = "variable_current_application"
	CurrentAppData = "variable_current_application_data"
	// application
	Application      = "Application"
	Application_Data = "Application-Data"
)

func (e Event) Name() EventName {
	en, err := EventNameString(e.Get(Event_Name))
	if err != nil {
		Warnf("parse event name error: %v", err)
		return CUSTOM
	}
	return en
}

func (e Event) FireTime() FireTime {
	ft, err := strconv.ParseInt(e.Get(Event_Date_Timestamp), 10, 64)
	if err != nil {
		Warnf("parse fire time error: %v", err)
		return 0
	}
	return FireTime(ft)
}

func (e Event) CoreUuid() string {
	return e.Get(Core_Uuid)
}

func (e Event) CallUuid() string {
	return e.Get(Unique_ID)
}

// app and data
func (e Event) App() (string, string) {
	return e.Get(Application_Data), e.Get(Application_Data)
}

// current app and data
func (e Event) CurrentApp() (string, string) {
	return e.Get(CurrentApp), e.Get(CurrentAppData)
}

func (e Event) Digits() string {
	return e.Get(DTMF_Digit)
}

func (e Event) DigitsSource() string {
	return e.Get(DTMF_Source)
}

func (e Event) CallDirection() string {
	return e.Get(Call_Direction)
}

func (e Event) Caller() string {
	return e.Get(Caller_ID_Number)
}

func (e Event) Callee() string {
	return e.Get(Callee_ID_Number)
}

func (e Event) Sequence() string {
	return e.Get(Event_Sequence)
}

func (e Event) SipFrom() string {
	return e.Get(Sip_From_Uri)
}

func (e Event) CoreNetworkAddr() string {
	return e.Get(Caller_Network_Addr)
}

func (e Event) CoreIP() string {
	return e.Get(FreeSWITCH_IPv4)
}

func (e Event) BgJob() string {
	return e.Get(BgJob_Uuid)
}

func (e Event) BgCommand() (string, string) {
	return e.Get(BgJob_Command), e.Get(BgJob_Command_Arg)
}

func (e Event) BgJobResponse() string {
	return e.GetTextBody()
}
