// Copyright 2022 genmzy. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goesl

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

type ConnHandler interface {
	OnConnect(*Connection)
	OnDisconnect(*Connection, Event)
	OnEvent(context.Context, *Connection, Event)
	OnClose(*Connection)
}

type Connection struct {
	// replaced when reset
	c         net.Conn
	waitings  []time.Duration //  all old waitings
	closeOnce *sync.Once      //  use pointer so that can be reset
	connected bool
	buffer    *bufio.ReadWriter

	opts *Options

	cmdReply chan Event
	apiResp  chan Event
	Handler  ConnHandler
	Address  string
	Password string

	srWg     sync.WaitGroup
	srCancel func()
	srTicker *time.Ticker
	// channel between send-reply and handle-events
	cmdReplyHandlers chan cmdReplyHandler
}

func (conn *Connection) dialTimes(retry bool) (err error) {
	if !retry {
		conn.c, err = net.DialTimeout("tcp", conn.Address, conn.opts.dialTimeout)
		return err
	}
	for i := 0; i <= conn.opts.maxRetries || conn.opts.maxRetries < 0; i++ {
		next := conn.opts.nextDialWait(conn.waitings)
		conn.waitings = append(conn.waitings, next)
		if len(conn.waitings) >= 2 {
			sleep := conn.waitings[len(conn.waitings)-2]
			conn.opts.logger.Warnf("connect failed, will retry in %v", sleep)
			time.Sleep(sleep)
		}
		conn.opts.logger.Debugf("start dial with timeout: %s", conn.opts.dialTimeout)
		conn.c, err = net.DialTimeout("tcp", conn.Address, conn.opts.dialTimeout)
		if err == nil {
			conn.waitings = conn.waitings[:0]
			break
		}
	}
	return
}

// reset all conn-related things
// if err is nil, that means dial first time
func (conn *Connection) reset(err error) error {
	if err == nil {
		conn.opts.logger.Infof("new connection dial connection %s start...",
			conn.Address)
	}
	if errors.Is(err, net.ErrClosed) || errors.Is(err, context.Canceled) {
		return fmt.Errorf("former error %v, should exit...", err)
	}
	retry := false
	if err != nil {
		retry = true
		conn.Close()
	}
	if err := conn.dialTimes(retry); err != nil {
		return err
	}

	conn.connected = true
	conn.closeOnce = new(sync.Once)
	conn.buffer = bufio.NewReadWriter(bufio.NewReaderSize(conn.c, 16*1024),
		bufio.NewWriter(conn.c))
	if err := conn.auth(); err != nil {
		return err
	}

	// create a send-reply goroutine
	conn.srWg.Add(1)
	ctx, cancel := context.WithCancel(context.Background())
	conn.srCancel = cancel
	go conn.sendReply(ctx)

	if conn.opts.autoRedial {
		conn.MustSendOK(ctx, "event", "plain", HEARTBEAT.String())
	}

	conn.Handler.OnConnect(conn)

	return nil
}

func emptyTickerChan(ticker *time.Ticker) {
	// loop seems uncessary, but still keep that
	for {
		select {
		case <-ticker.C:
		default:
			return
		}
	}
}

// goroutine send-reply to send command and receive reply/response
func (conn *Connection) sendReply(ctx context.Context) {
	defer conn.srWg.Done()
	for {
		var crh cmdReplyHandler
		select {
		case <-ctx.Done():
			conn.opts.logger.Noticef("send-reply of %s cancel with error: %v\n",
				conn.Address, ctx.Err())
			return
		case crh = <-conn.cmdReplyHandlers:
		}

		if err := conn.c.SetWriteDeadline(time.Now().Add(conn.opts.netDelay)); err != nil {
			// conn is closing or deadline is 0
			conn.opts.logger.Errorf("set write deadline: %v", err)
		}

		// NOTE: if arrive write deadline, socket may already write some bytes
		_, err := conn.write(crh.cmd)
		if err != nil {
			// cannot detect write timeout, write will not waiting for tcp ACK
			if lost, _ := connLost(err, 'w'); lost {
				conn.opts.logger.Errorf("write error, send-reply exiting with err: %v", err)
				goto connLossAccident
			}
			// TODO: need to be tested <2022-04-25, genmzy> //
			conn.opts.logger.Warnf("write error: %v, continuing...", err)
			continue
		}

		// empty ticker channel to avoid former arrived ticks
		emptyTickerChan(conn.srTicker)

		var ev Event
		ivals := 0
	waitForReply:
		for {
			select {
			case <-ctx.Done():
				conn.opts.logger.Noticef("send-reply of %s cancel with error: %v\n", conn.Address, ctx.Err())
				return
			case ev = <-conn.cmdReply:
				break waitForReply
			case ev = <-conn.apiResp:
				break waitForReply
			case <-conn.srTicker.C:
				// NOTE: 2 times ticker receive here for keeping at least 1 net delay interval
				if ivals >= 2 {
					conn.opts.logger.Debugf("receive 2 intervals, jumping to connLossAccident...")
					goto connLossAccident
				}
				ivals++
			}
		}
		if crh.rh != nil {
			crh.rh(ev, err)
		}
	}

connLossAccident:
	conn.opts.logger.Debugf("reach connection loss accident")
	// cancel read immediately, use any time.Time before or equals current time
	// conn.c.SetReadDeadline(time.Now()) will make one more system call (?)
	err := conn.c.SetReadDeadline(time.Unix(0, 0))
	if err != nil {
		conn.opts.logger.Errorf("set read deadline: %v", err)
	}
	return
}

// Create a new event socket connection and take a ConnectionHandler interface.
// This will create a new 'send-reply' goroutine this goroutine will exit when call
// goesl.*Connection.Close()
func Dial(addr, passwd string, handler ConnHandler, options ...Option) (*Connection, error) {
	conn := Connection{
		Address:  addr,
		Password: passwd,
		Handler:  handler,
	}
	conn.opts = newOptions(options)
	conn.waitings = make([]time.Duration, 0, conn.opts.maxRetries+1)

	conn.cmdReply = make(chan Event, conn.opts.sendReplyCap)
	conn.apiResp = make(chan Event, conn.opts.sendReplyCap)
	conn.cmdReplyHandlers = make(chan cmdReplyHandler, conn.opts.sendReplyCap)
	conn.srTicker = time.NewTicker(conn.opts.netDelay)

	// do connect
	err := conn.reset(nil)
	if err != nil {
		return nil, err
	}

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
	crh := cmdReplyHandler{
		cmd: buf.Bytes(),
		rh:  h,
	}
	select {
	case conn.cmdReplyHandlers <- crh:
	case <-ctx.Done():
	}
}

// must send and receive command, or fatal the process
func (conn *Connection) MustSendOK(ctx context.Context, cmd string, args ...string) {
	f := func(err error) {
		if err == nil {
			return
		}
		conn.opts.logger.Fatalf(
			fmt.Sprintf("with cmd: %s(%v) error: %v\n",
				cmd, args, err,
			),
		)
	}
	s := RepJustCareError{
		CHandle: f,
		RHandle: f,
	}
	conn.Send(ctx, s.RepHandle, cmd, args...)
}

// Send event to FreeSWITCH, this is NOT a API or BgAPI command
// suggest: use RepJustCareError
func (conn *Connection) SendEvent(ctx context.Context, h RepHandler, evName string,
	headers map[string]string, body []byte) {
	buf := bytes.NewBufferString("sendevent ")
	buf.WriteString(evName)
	buf.WriteString("\r\n")
	for k, v := range headers {
		buf.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	buf.WriteString(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body)))
	buf.Write(body)

	crh := cmdReplyHandler{
		cmd: buf.Bytes(),
		rh:  h,
	}
	select {
	case conn.cmdReplyHandlers <- crh:
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
func (conn *Connection) ExecuteLooped(ctx context.Context, h RepHandler, app string, uuid string,
	loops uint, params ...string) {
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
func (conn *Connection) ExecuteLoopedSync(ctx context.Context, h RepHandler, app string, uuid string,
	loops uint, params ...string) {
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

func (conn *Connection) makeBlock() {
	conn.opts.logger.Debugf("set connection %s to block", conn.Address)
	conn.c.SetDeadline(time.Time{})
}

// auth handles freeswitch esl authentication
func (conn *Connection) auth() error {
	ev, err := conn.recvEvent()
	if err != nil || ev.Type != EventAuth {
		conn.c.Close()
		if ev.Type != EventAuth {
			return fmt.Errorf("bad auth preamble: [%s]", ev.header)
		}
		return fmt.Errorf("socket read error: %v", err)
	}

	var buf bytes.Buffer
	buf.WriteString("auth ")
	buf.WriteString(conn.Password)
	buf.WriteString("\r\n\r\n")

	conn.opts.logger.Debugf("set auth deadline for dial timeout %v", conn.opts.dialTimeout)
	if err := conn.c.SetDeadline(time.Now().Add(conn.opts.dialTimeout)); err != nil {
		return fmt.Errorf("set deadline after %v: %v", conn.opts.dialTimeout, err)
	}
	defer conn.makeBlock()
	if _, err := conn.write(buf.Bytes()); err != nil {
		conn.c.Close()
		return fmt.Errorf("passwd buffer flush: %v", err)
	}

	ev, err = conn.recvEvent()
	if err != nil {
		conn.c.Close()
		return fmt.Errorf("auth reply: %v", err)
	}
	if ev.Type != EventCommandReply {
		conn.c.Close()
		return fmt.Errorf("bad reply type: %#v", ev.Type)
	}
	if reply := ev.Get("Reply-Text"); strings.HasPrefix(reply, "-ERR ") {
		return fmt.Errorf("auth reply: %v", reply[5:])
	}
	return nil
}

// return value: { is_connection_error, is_connection_error_by_accident }
// never { false, true }
func connLost(err error, mode byte) (bool, bool) {
	switch mode {
	case 'r': // read
		if errors.Is(err, os.ErrDeadlineExceeded) {
			return true, true
		}
		if errors.Is(err, io.EOF) {
			return true, true
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return true, true
		}
		if errors.Is(err, net.ErrClosed) {
			return true, false
		}
	case 'w': // write
		if errors.Is(err, os.ErrDeadlineExceeded) {
			return true, true
		}
		if errors.Is(err, syscall.EPIPE) {
			return true, true
		}
		if errors.Is(err, net.ErrClosed) {
			return true, false
		}
	}
	return false, false
}

// Receive events and handle them by `goesl.ConnectionHandler`
// if error returned is not `os.ErrClosed` or `context.ErrCanceled`
// that means unexpected error occurred. To distingush that, you can use:
// `errors.Is(err, net.ErrClosed) || errors.Is(err, context.Canceled)`
func (conn *Connection) HandleEvents(ctx context.Context) error {
	defer func() {
		conn.opts.logger.Noticef("waiting for send-reply exiting...")
		if !conn.opts.autoRedial && conn.connected { // this is a timeout exception
			conn.opts.logger.Infof("connection disable automatic redial, so call send-reply cancel function.")
			conn.srCancel()
		}
		conn.srWg.Wait()
		conn.opts.logger.Noticef("waiting for send-reply exiting done.")
	}()
	for conn.connected {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		ev, err := conn.recvEvent()
		if err != nil {
			if lost, accident := connLost(err, 'r'); lost {
				if conn.opts.autoRedial && accident {
					if err := conn.reset(err); err != nil {
						return err
					}
					continue
				}
				return err
			}
			conn.opts.logger.Warnf("recvEvent: %v, continuing...", err)
			continue
		}
		switch ev.Type {
		case EventInvalid:
			conn.opts.logger.Errorf("invalid event: [%s]", ev)
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

// NOTE: an error impletion:
//    defer conn.buffer.Flush()
//    return conn.buffer.Write()
// this will ignore the real connection write error that in `Flush`
func (conn *Connection) write(b []byte) (int, error) {
	n, err := conn.buffer.Write(b)
	if err != nil {
		return n, err
	}
	err = conn.buffer.Flush()
	return n, err
}

// Close the connection and make send-reply exit as soon as possible
// ignore all errors here because may the connection already lost
//
// Close is protected by sync.Once, so you can call this for twice or more
func (conn *Connection) Close() {
	conn.closeOnce.Do(func() {
		if conn.connected {
			conn.connected = false
			conn.Handler.OnClose(conn)
		}
		conn.srCancel() // close send-reply goroutine
		// cancel write immediately, use any time.Time before or equals current time
		// conn.c.SetReadDeadline(time.Now()) will make one more system call (?)
		conn.c.SetDeadline(time.Unix(0, 0))
		conn.c.Close()
	})
}

func (conn *Connection) recvEvent() (Event, error) {
	var err error
	e := Event{Type: EventInvalid}
	r := conn.buffer.Reader

	if conn.opts.autoRedial {
		rd := conn.opts.heartbeat + conn.opts.netDelay
		// conn.opts.logger.Debugf("automatic redial enable with heartbeat, set read deadline %v", rd)
		err = conn.c.SetReadDeadline(time.Now().Add(rd))
		if err != nil {
			// conn is closing or ctx is 0, just return
			return e, err
		}
	}

	e.header.headers, err = textproto.NewReader(r).ReadMIMEHeader()
	if err != nil {
		return e, err
	}

	if slen := e.Get("Content-Length"); slen != "" {
		len, err := strconv.Atoi(slen)
		if err != nil {
			return e, fmt.Errorf("convert content-length %s: %v", slen, err)
		}
		e.rawBody = make([]byte, len)
		_, err = io.ReadFull(r, e.rawBody)
		if err != nil {
			return e, fmt.Errorf("read body: %v", err)
		}
	}

	switch t := e.Get("Content-Type"); t {
	case "auth/request":
		e.Type = EventAuth
	case "command/reply":
		e.Type = EventCommandReply
		reply := e.Get("Reply-Text")
		if strings.Contains(reply, "%") {
			e.header.escaped = true
		}
	case "text/event-plain":
		e.Type = EventGeneric
		err = e.parseTextBody()
	case "text/event-json", "text/event-xml":
		err = fmt.Errorf("unsupported format %s", t)
	case "text/disconnect-notice", "text/rude-rejection":
		e.Type = EventDisconnect
	case "api/response":
		e.Type = EventApiResponse
	default:
		e.Type = EventInvalid
	}
	return e, err
}
