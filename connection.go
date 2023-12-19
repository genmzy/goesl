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

	"github.com/genmzy/goesl/ev_name"

	"github.com/google/uuid"
)

type ConnHandler interface {
	OnConnect(*Connection)
	OnDisconnect(*Connection, Event)
	OnEvent(context.Context, *Connection, Event)
	OnFslog(context.Context, *Connection, Fslog)
	OnClose(*Connection)
}

type Connection struct {
	// reset when connection auto redial
	Handler            ConnHandler
	c                  net.Conn
	connected          bool
	closeOnce          *sync.Once
	buffer             *bufio.ReadWriter
	evCallbackCanceler func()

	// avoid write in one goroutine and flush in another goroutine, which names `short write` error
	writeLock *sync.Mutex
	srWg      sync.WaitGroup

	cmdReply chan Event // channel to wait command send reply
	apiResp  chan Event // channel to wait api command send response
	events   chan Event // channel to send plain events

	opts *Options

	Address  string
	Password string
}

// `retry` just a flag to indicate that if this is a retry dial
func (conn *Connection) dialTimes(retry bool) (err error) {
	if !retry {
		conn.c, err = net.DialTimeout("tcp", conn.Address, conn.opts.dialTimeout)
		return
	}
	for i := 0; i <= conn.opts.maxRetries || conn.opts.maxRetries < 0; i++ {
		conn.opts.logger.Debugf("start dial with timeout: %s", conn.opts.dialTimeout)
		conn.c, err = net.DialTimeout("tcp", conn.Address, conn.opts.dialTimeout)
		if err == nil {
			if i > 0 {
				conn.opts.redialStrategy.Reset()
			}
			return
		}
		next := conn.opts.redialStrategy.NextRedoWait()
		conn.opts.logger.Warnf("connect failed, will retry in %v", next)
		time.Sleep(next)
	}
	return
}

// reset all conn-related things
// if parameter `former` is nil, that means dial first time
func (conn *Connection) reset(former error) error {
	if former == nil {
		conn.opts.logger.Infof("new connection dial %s start...", conn.Address)
	}
	if errors.Is(former, net.ErrClosed) || errors.Is(former, context.Canceled) {
		return fmt.Errorf("former error: %v", former)
	}
	retry := false
	if former != nil {
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

	// create a event callback goroutine
	conn.srWg.Add(1)
	ctx, cancel := context.WithCancel(context.Background())
	conn.evCallbackCanceler = cancel
	go conn.eventCallback(ctx)

	return nil
}

func (conn *Connection) eventCallback(ctx context.Context) {
	if conn.opts.autoRedial {
		conn.Plain(ctx, []ev_name.EventName{ev_name.HEARTBEAT}, nil)
	}
	conn.Handler.OnConnect(conn)

	defer conn.srWg.Done()
forloop:
	for {
		var ev Event
		select {
		case <-ctx.Done():
			conn.opts.logger.Noticef("event callback of %s cancel with error: %v\n",
				conn.Address, ctx.Err())
			break forloop
		case ev = <-conn.events:
		}
		conn.Handler.OnEvent(ctx, conn, ev)
	}
	conn.opts.logger.Noticef("listen events of %s cancel with error: %v\n",
		conn.Address, ctx.Err())
}

// Create a new event socket connection and take a ConnectionHandler interface.
// This will create a new 'event callback' goroutine this goroutine will exit when call
// goesl.*Connection.Close()
func Dial(addr, passwd string, handler ConnHandler, options ...Option) (*Connection, error) {
	conn := Connection{
		Address:  addr,
		Password: passwd,
		Handler:  handler,
	}
	conn.opts = newOptions(options)

	conn.cmdReply = make(chan Event, conn.opts.sendReplyCap)
	conn.apiResp = make(chan Event, conn.opts.sendReplyCap)
	conn.events = make(chan Event, conn.opts.sendReplyCap)
	conn.writeLock = &sync.Mutex{}

	// NOTE: only for default logger and
	// should set output first to clear if prefix should be colored
	if conn.opts.logOutput != nil {
		defaultLogger.setOutput(conn.opts.logOutput)
	}
	if conn.opts.logPrefix != "" {
		defaultLogger.setPrefix(conn.opts.logPrefix)
	}

	// do connect
	err := conn.reset(nil)
	if err != nil {
		return nil, err
	}

	return &conn, nil
}

// Send to FreeSWITCH and handle result(error and event)
func (conn *Connection) Send(ctx context.Context, cmd string, args ...string) (string, error) {
	buf := bytes.NewBufferString(cmd)
	for _, arg := range args {
		buf.WriteString(" ")
		buf.WriteString(arg)
	}
	buf.WriteString("\r\n\r\n")
	if err := conn.c.SetWriteDeadline(time.Now().Add(conn.opts.netDelay)); err != nil {
		// conn is closing or deadline is 0
		conn.opts.logger.Errorf("set write deadline: %v", err)
	}
	return conn.SendBytes(ctx, buf.Bytes())
}

func (conn *Connection) SendBytes(ctx context.Context, buf []byte) (string, error) {
	_, err := conn.write(buf)
	if err != nil {
		// cannot detect write timeout, write will not waiting for tcp ACK
		if lost, _ := connLost(err, 'w'); lost {
			conn.opts.logger.Errorf("write error, event callback exiting with err: %v", err)
			// cancel read immediately, use any time.Time before or equals current time
			// conn.c.SetReadDeadline(time.Now()) will make one more system call
			err = conn.c.SetReadDeadline(time.Unix(0, 0))
			if err != nil {
				conn.opts.logger.Errorf("set read deadline: %v", err)
			}
		} else {
			conn.opts.logger.Warnf("write error: %v, continuing...", err)
		}
		return "", err
	}
	var ev Event
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case ev = <-conn.cmdReply:
	case ev = <-conn.apiResp:
	}
	return ev.ErrOrRes()
}

// must send and receive command, or fatal the process
func (conn *Connection) MustSendOK(ctx context.Context, cmd string, args ...string) {
	_, err := conn.Send(ctx, cmd, args...)
	if err != nil {
		conn.opts.logger.Fatalf("with cmd: %s(%v) error: %v\n", cmd, args, err)
	}
}

func (conn *Connection) Plain(ctx context.Context, ens []ev_name.EventName, subs []string) {
	strs := make([]string, 0)
	strs = append(strs, "plain")
	for _, en := range ens {
		// enStrs = append(enStrs, en.String())
		strs = append(strs, en.String())
	}
	if len(subs) != 0 {
		strs = append(strs, ev_name.CUSTOM.String())
	}
	strs = append(strs, subs...)
	conn.MustSendOK(ctx, "event", strs...)
}

func (conn *Connection) Fslog(ctx context.Context, lv FslogLevel) {
	conn.MustSendOK(ctx, "log", lv.String())
}

// Send event to FreeSWITCH, this is NOT a API or BgAPI command
// suggest: use RepJustCareError
func (conn *Connection) SendEvent(ctx context.Context, en ev_name.EventName, headers map[string]string, body []byte) error {
	buf := bytes.NewBufferString("sendevent ")
	buf.WriteString(en.String())
	buf.WriteString("\r\n")
	for k, v := range headers {
		fmt.Fprintf(buf, "%s: %s\r\n", k, v)
	}
	fmt.Fprintf(buf, "Content-Length: %d\r\n\r\n", len(body))
	buf.Write(body)
	_, err := conn.SendBytes(ctx, buf.Bytes())
	return err
}

// Send API command to FreeSWITCH by event socket, already start with `api `
// get result in param `h` by calling method, esl.Event.GetTextBody(), result maybe start withh `-ERR `.
// NOTE: do not use block API such as orignate here, which will block fs from sending other events(e.g.
// HEARTBEAT, further make esl client automatic redial), so use (*Connection).BgApi
func (conn *Connection) Api(ctx context.Context, cmd string, args ...string) (string, error) {
	cmd = fmt.Sprintf("api %s", cmd)
	return conn.Send(ctx, cmd, args...)
}

// `bgapi` command will never response error, so just care Connection error handle
// This is a better way to use `api bgapi uuid:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx` instead of `bgapi `
// and wait for job uuid
func (conn *Connection) BgApi(ctx context.Context, cmd string, args ...string) (string, error) {
	bgJob := uuid.New().String()
	cmd = fmt.Sprintf("api bgapi uuid:%s %s", bgJob, cmd)
	_, err := conn.Send(ctx, cmd, args...)
	return bgJob, err
}

// Execute an app on leg `uuid` and for `loops` times
// suggest: use RepJustCareError
func (conn *Connection) ExecuteLooped(ctx context.Context, app string, uuid string, loops uint, params ...string) (string, error) {
	args := strings.Join(params, " ")
	cmd := Command{
		Sync:  false,
		Uuid:  uuid,
		App:   app,
		Args:  args,
		Loops: loops,
	}
	return cmd.Execute(ctx, conn)
}

// Execute an app on leg `uuid` and for `loops` times, this app will not be interrupted util finish.
// suggest: use RepJustCareError
func (conn *Connection) ExecuteLoopedSync(ctx context.Context, app string, uuid string,
	loops uint, params ...string,
) (string, error) {
	args := strings.Join(params, " ")
	cmd := Command{
		Sync:  true,
		Uuid:  uuid,
		App:   app,
		Args:  args,
		Loops: loops,
	}
	return cmd.Execute(ctx, conn)
}

// Execute an app on leg `uuid`
// suggest: use RepJustCareError
func (conn *Connection) Execute(ctx context.Context, app string, uuid string, params ...string) (string, error) {
	args := strings.Join(params, " ")
	cmd := Command{
		Sync:  false,
		Uuid:  uuid,
		App:   app,
		Args:  args,
		Loops: 1,
	}
	return cmd.Execute(ctx, conn)
}

// Execute an app on leg `uuid`, this app will not be interrupted util finish.
// suggest: use RepJustCareError
func (conn *Connection) ExecuteSync(ctx context.Context, app string, uuid string, params ...string) (string, error) {
	args := strings.Join(params, " ")
	cmd := Command{
		Sync:  true,
		Uuid:  uuid,
		App:   app,
		Args:  args,
		Loops: 1,
	}
	return cmd.Execute(ctx, conn)
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
		conn.opts.logger.Noticef("waiting for event callback goroutine exiting...")
		if !conn.opts.autoRedial && conn.connected { // this is a timeout exception
			conn.opts.logger.Infof("connection disable automatic redial, so execute event callback canceler.")
			conn.evCallbackCanceler()
		}
		conn.srWg.Wait()
		conn.opts.logger.Noticef("waiting for event callback exiting done.")
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
			conn.opts.logger.Warnf("receive event: %v, continuing ...", err)
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
			conn.events <- ev
		case EventLog:
			fslog, err := Event2Fslog(ev)
			if err != nil {
				conn.opts.logger.Errorf("log event convert: %v", err)
				continue
			}
			conn.Handler.OnFslog(ctx, conn, fslog)
		}
	}
	return fmt.Errorf("disconnected")
}

// NOTE: an error implementation:
//
// defer conn.buffer.Flush()
// return conn.buffer.Write()
//
// this will ignore the real connection write error that in `Flush`
func (conn *Connection) write(b []byte) (int, error) {
	// conn.opts.logger.Noticef("write: %s", string(b))
	conn.writeLock.Lock()
	defer conn.writeLock.Unlock()
	n, err := conn.buffer.Write(b)
	if err != nil {
		return n, err
	}
	err = conn.buffer.Flush()
	return n, err
}

// Close the connection and make event callback exit as soon as possible
// ignore all errors here because may the connection already lost
//
// Close is protected by sync.Once, so you can call this for twice or more
func (conn *Connection) Close() {
	conn.closeOnce.Do(func() {
		if conn.connected {
			conn.connected = false
			conn.Handler.OnClose(conn)
		}
		conn.evCallbackCanceler() // make event callback goroutine exit
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
		conn.opts.logger.Debugf("read deadline arrived: %v...", errors.Is(err, os.ErrDeadlineExceeded))
		return e, err
	}

	if slen := e.Get("Content-Length"); slen != "" {
		l, err := strconv.Atoi(slen)
		if err != nil {
			return e, fmt.Errorf("convert content-length %s: %v", slen, err)
		}
		e.rawBody = make([]byte, l)
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
	case "log/data":
		e.Type = EventLog
	default:
		e.Type = EventInvalid
	}
	// conn.opts.logger.Noticef("recvEvent: %v", e)
	return e, err
}
