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

	"github.com/genmzy/goesl/ev_header"
	"github.com/genmzy/goesl/ev_name"
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

func (m mimeMap) getRaw(key string) string {
	return m.headers.Get(key)
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
	header  mimeMap
	body    mimeMap
	rawBody []byte

	Type EventType
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
	EventLog
	EventGeneric
)

func (e Event) GetTextBody() string {
	if e.Type == EventApiResponse || e.Type == EventLog {
		resp := strings.TrimSpace(string(e.rawBody))
		return string(resp)
	}
	slen := e.body.get("Content-Length")
	if slen == "" {
		return ""
	}
	bblen, err := strconv.Atoi(slen)
	if err != nil {
		// FIXME: this will never be called by a replaced logger, but keep this
		Warnf("content length parse error: %v", err)
		return ""
	}
	blen := len(e.rawBody)
	return string(e.rawBody[blen-bblen : blen-1])
}

// Get retrieves the value of header from Event header or (if not found) from Event body.
// The value is returned unescaped and is empty if not found anywhere.
func (e Event) Get(header string) string {
	val := e.body.get(header)
	if val == "" {
		val = e.header.get(header)
	}
	return val
}

func (e Event) GetRaw(header string) string {
	val := e.body.getRaw(header)
	if val == "" {
		val = e.header.get(header)
	}
	return val
}

func (e Event) String() string {
	body, _ := url.QueryUnescape(string(e.rawBody))
	return fmt.Sprintf("%v\n.\n%v====================\n", e.header, body)
}

func (e Event) EventContent() string {
	if e.Type == EventGeneric {
		return string(e.rawBody)
	}
	return ""
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

func (e Event) Name() ev_name.EventName {
	en, err := ev_name.EventNameString(e.Get(ev_header.Event_Name))
	if err != nil {
		// FIXME: this will never be called by a replaced logger, but keep this
		Warnf("parse event name error: %v", err)
		return ev_name.CUSTOM
	}
	return en
}

func (e Event) FireTime() FireTime {
	ft, err := strconv.ParseInt(e.Get(ev_header.Event_Date_Timestamp), 10, 64)
	if err != nil {
		// FIXME: this will never be called by a replaced logger, but keep this
		Warnf("parse fire time error: %v", err)
		return 0
	}
	return FireTime(ft)
}

func (e Event) BgJobResponse() string {
	return e.GetTextBody()
}

func (e Event) ErrOrRes() (raw string, err error) {
	switch e.Type {
	case EventCommandReply:
		raw = e.Get("Reply-Text")
	case EventApiResponse:
		raw = e.GetTextBody()
	default:
		return
	}
	if len(raw) < 6 {
		return
	}
	if strings.HasPrefix(raw, "-ERR ") {
		return raw, errors.New(raw[5:])
	}
	return raw, nil
}

func (e Event) CoreUuid() string {
	return e.Get(ev_header.Core_Uuid)
}

func (e Event) Uuid() string {
	return e.Get(ev_header.Unique_ID)
}

// app and data
func (e Event) App() (string, string) {
	return e.Get(ev_header.Application_Data), e.Get(ev_header.Application_Data)
}

func (e Event) API() (string, string) {
	return e.Get(ev_header.API_Command), e.Get(ev_header.API_Command_Argument)
}

// current app and data
func (e Event) CurrentApp() (string, string) {
	return e.Get(ev_header.CurrentApp), e.Get(ev_header.CurrentAppData)
}

func (e Event) Digits() string {
	return e.Get(ev_header.DTMF_Digit)
}

func (e Event) DigitsSource() string {
	return e.Get(ev_header.DTMF_Source)
}

func (e Event) CallDirection() string {
	return e.Get(ev_header.Call_Direction)
}

func (e Event) Caller() string {
	return e.Get(ev_header.Caller_ID_Number)
}

func (e Event) Callee() string {
	return e.Get(ev_header.Callee_Destination_Number)
}

func (e Event) Sequence() string {
	return e.Get(ev_header.Event_Sequence)
}

func (e Event) SipFrom() string {
	return e.Get(ev_header.Sip_From_Host)
}

func (e Event) SipTo() string {
	return e.Get(ev_header.Sip_To_Host)
}

func (e Event) CoreNetworkAddr() string {
	return e.Get(ev_header.Caller_Network_Addr)
}

func (e Event) CoreIP() string {
	return e.Get(ev_header.FreeSWITCH_IPv4)
}

func (e Event) BgJob() string {
	return e.Get(ev_header.BgJob_Uuid)
}

func (e Event) BgCommand() (string, string) {
	return e.Get(ev_header.BgJob_Command), e.Get(ev_header.BgJob_Command_Arg)
}
