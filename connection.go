// Copyright 2022 genmzy. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goesl

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ConnHandler interface {
	OnConnect(*Connection)
	OnDisconnect(*Connection, *Event)
	OnEvent(context.Context, *Connection, *Event)
	OnClose(*Connection)
}

type Connection struct {
	socket     net.Conn
	buffer     *bufio.ReadWriter
	cmdReply   chan *Event
	apiResp    chan *Event
	Handler    ConnHandler
	Address    string
	Password   string
	Connected  bool
	MaxRetries int
	Timeout    time.Duration
	UserData   interface{}

	senderCancel    func()
	sendRepHandlers chan sendRepHandler

	logger EslLogger
}

// Create a new event socket connection and take a ConnectionHandler interface.
// This will create a new 'sender' goroutine this goroutine will exit when call
// goesl.Connection.Close()
func NewConnection(host, passwd string, handler ConnHandler) (*Connection, error) {
	conn := Connection{
		Address:  host,
		Password: passwd,
		Timeout:  3 * time.Second,
		Handler:  handler,
	}
	conn.cmdReply = make(chan *Event, 10)
	conn.apiResp = make(chan *Event, 10)
	conn.sendRepHandlers = make(chan sendRepHandler, 10)
	conn.logger = log.Default()

	// create a sender
	ctx, cancel := context.WithCancel(context.Background())
	conn.senderCancel = cancel
	go func(ctx context.Context) {
		for {
			var srh sendRepHandler
			select {
			case <-ctx.Done():
				conn.logger.Printf("[goesl.NewConnection] sender of %s exiting with error: %v\n", conn.Address, ctx.Err())
				return
			case srh = <-conn.sendRepHandlers:
			}
			_, err := conn.Write(srh.Content)
			if err != nil {
				err = &Error{
					Original: err,
					Stack:    fmt.Sprintf("send command %s", string(srh.Content)),
				}
				// TODO: handle error here <2022-04-11, genmzy> //
				conn.logger.Printf("[goesl.NewConnection] error: %v, will continue", err)
				continue
			}
			var ev *Event
			select {
			case <-ctx.Done():
				conn.logger.Printf("[goesl.NewConnection] sender of %s exiting with error: %v\n", conn.Address, ctx.Err())
				return
			case ev = <-conn.cmdReply:
			case ev = <-conn.apiResp:
			}
			if srh.Handler != nil {
				srh.Handler(ev, err)
			}
		}
	}(ctx)

	err := conn.connectRetry(3)
	if err != nil {
		return nil, fmt.Errorf("connect: %v", err)
	}
	conn.Handler.OnConnect(&conn)
	return &conn, nil
}

// Send to FreeSWITCH and handle result(error and event)
func (conn *Connection) Send(ctx context.Context, h RepHandler, cmd string, args ...string) {
	buf := bytes.NewBufferString(cmd)
	for _, arg := range args {
		buf.WriteString(" ")
		buf.WriteString(arg)
	}
	buf.WriteString("\r\n\r\n")
	srh := sendRepHandler{
		Content: buf.Bytes(),
		Handler: h,
	}
	select {
	case conn.sendRepHandlers <- srh:
	case <-ctx.Done():
	}
}

// must send and receive command, or fatal the process
func (conn *Connection) MustSendOK(ctx context.Context, cmd string, args ...string) {
	f := func(err error) {
		if err != nil {
			return
		}
		conn.logger.Fatal(fmt.Sprintf("[esl.Connection.MustSendOK] with cmd: %s(%v) error: %v\n", cmd, args, err))
	}
	s := RepJustCareError{
		CHandle: f,
		RHandle: f,
	}
	conn.Send(ctx, s.RepHandle, cmd, args...)
}

// Send event to FreeSWITCH, this is NOT a API or BgAPI command
// suggest: use RepJustCareError
func (conn *Connection) SendEvent(ctx context.Context, h RepHandler, evName string, headers map[string]string, body []byte) {
	buf := bytes.NewBufferString("sendevent ")
	buf.WriteString(evName)
	buf.WriteString("\r\n")
	for k, v := range headers {
		buf.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	buf.WriteString(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body)))
	buf.Write(body)

	srh := sendRepHandler{
		Content: buf.Bytes(),
		Handler: h,
	}
	select {
	case conn.sendRepHandlers <- srh:
	case <-ctx.Done():
	}
}

// Send API command to FreeSWITCH by event socket, already start with `api `
// get result in param `h` by calling method, esl.Event.GetTextBody(), result maybe start withh `-ERR `
func (conn *Connection) Api(ctx context.Context, h RepHandler, cmd string, args ...string) {
	cmd = fmt.Sprintf("api %s", cmd)
	conn.Send(ctx, h, cmd, args...)
}

// `bgapi` command will never response error, so just care Connection error handle
// This is a better way to use `api bgapi uuid:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx` instead of `bgapi `
// and wait for job uuid
func (conn *Connection) BgApi(ctx context.Context, h ConnErrHandler, cmd string, args ...string) string {
	bgJob := uuid.New().String()
	cmd = fmt.Sprintf("api bgapi uuid:%s %s", bgJob, cmd)
	r := RepBgUuid{CHandle: h}
	conn.Send(ctx, r.RepHandle, cmd, args...)
	return bgJob
}

// Execute an app on leg `uuid` and for `loops` times
// suggest: use RepJustCareError
func (conn *Connection) ExecuteLooped(ctx context.Context, h RepHandler, app string, uuid string, loops uint, params ...string) {
	args := strings.Join(params, " ")
	cmd := Command{
		Sync:  false,
		UId:   uuid,
		App:   app,
		Args:  args,
		Loops: loops,
	}
	cmd.Execute(ctx, conn, h)
}

// Execute an app on leg `uuid` and for `loops` times, this app will not be interrupted util finish.
// suggest: use RepJustCareError
func (conn *Connection) ExecuteLoopedSync(ctx context.Context, h RepHandler, app string, uuid string, loops uint, params ...string) {
	args := strings.Join(params, " ")
	cmd := Command{
		Sync:  true,
		UId:   uuid,
		App:   app,
		Args:  args,
		Loops: loops,
	}
	cmd.Execute(ctx, conn, h)
}

// Execute an app on leg `uuid`
// suggest: use RepJustCareError
func (conn *Connection) Execute(ctx context.Context, h RepHandler, app string, uuid string, params ...string) {
	args := strings.Join(params, " ")
	cmd := Command{
		Sync:  false,
		UId:   uuid,
		App:   app,
		Args:  args,
		Loops: 1,
	}
	cmd.Execute(ctx, conn, h)
}

// Execute an app on leg `uuid`, this app will not be interrupted util finish.
// suggest: use RepJustCareError
func (conn *Connection) ExecuteSync(ctx context.Context, h RepHandler, app string, uuid string, params ...string) {
	args := strings.Join(params, " ")
	cmd := Command{
		Sync:  true,
		UId:   uuid,
		App:   app,
		Args:  args,
		Loops: 1,
	}
	cmd.Execute(ctx, conn, h)
}

func (conn *Connection) connectRetry(mretries int) error {
	for retries := 1; !conn.Connected && retries <= mretries; retries++ {
		c, err := net.DialTimeout("tcp", conn.Address, conn.Timeout)
		if err != nil {
			if retries == mretries {
				return fmt.Errorf("last dial attempt: %v", err)
			}
			conn.logger.Printf("[goesl.Connection.ConnectRetry] dial attempt #%d: %v, retrying\n", retries, err)
		} else {
			conn.socket = c
			break
		}
	}
	conn.buffer = bufio.NewReadWriter(bufio.NewReaderSize(conn.socket, 16*1024),
		bufio.NewWriter(conn.socket))
	return conn.authenticate()
}

// authenticate handles freeswitch esl authentication
func (conn *Connection) authenticate() error {
	ev, err := NewEventFromReader(conn.buffer.Reader)
	if err != nil || ev.Type != EventAuth {
		conn.socket.Close()
		if ev.Type != EventAuth {
			return fmt.Errorf("bad auth preamble: [%s]", ev.Header)
		}
		return fmt.Errorf("socket read error: %v", err)
	}

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("auth %s\r\n\r\n", conn.Password))
	if _, err := conn.Write(buf.Bytes()); err != nil {
		conn.socket.Close()
		return fmt.Errorf("passwd buffer flush: %v", err)
	}

	ev, err = NewEventFromReader(conn.buffer.Reader)
	if err != nil {
		conn.socket.Close()
		return fmt.Errorf("auth reply: %v", err)
	}
	if ev.Type != EventCommandReply {
		conn.socket.Close()
		return fmt.Errorf("bad reply type: %#v", ev.Type)
	}
	conn.Connected = true
	return nil
}

// Receive events and handle them by `esl.ConnectionHandler` if the event is
func (conn *Connection) HandleEvents(ctx context.Context) error {
	for conn.Connected {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		ev, err := NewEventFromReader(conn.buffer.Reader)
		if err != nil {
			if rErr, ok := err.(*Error); ok {
				rErr.WithStack("event read loop")
				return rErr
			}
			// TODO: may be a handler here <2022-04-12, genmzy> //
			conn.logger.Printf("[goesl.Connection.HandleEvents] new event from reader: %v", err)
			continue
		}
		switch ev.Type {
		case EventInvalid:
			return fmt.Errorf("invalid event: [%s]", ev)
		case EventDisconnect:
			conn.Handler.OnDisconnect(conn, ev)
		case EventCommandReply:
			select {
			case conn.cmdReply <- ev:
			case <-ctx.Done():
				return ctx.Err()
			}
		case EventApiResponse:
			select {
			case conn.apiResp <- ev:
			case <-ctx.Done():
				return ctx.Err()
			}
		case EventGeneric:
			conn.Handler.OnEvent(ctx, conn, ev)
		}
	}
	return fmt.Errorf("disconnected")
}

func (conn *Connection) Write(b []byte) (int, error) {
	defer conn.buffer.Flush()
	return conn.buffer.Write(b)
}

func (conn *Connection) Close() {
	if conn.Connected {
		conn.Connected = false
		conn.Handler.OnClose(conn)
	}
	conn.senderCancel() // close sender goroutine
	conn.socket.Close()
}

// set logger for print esl package inner message output
func (conn *Connection) SetLogger(l EslLogger) {
	conn.logger = l
}
