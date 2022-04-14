package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	. "github.com/genmzy/goesl"
)

type Handler struct {
	CallId  string
	BgJobId string
}

const (
	Caller = "1002"
	Callee = "1001"
)

func main() {
	handler := &Handler{}
	conn, err := NewConnection("127.0.0.1:8021", "ClueCon", handler)
	if err != nil {
		log.Fatal("ERR connecting to freeswitch:", err)
	}
	conn.SetLogger(log.Default())
	ctx, cancel := context.WithCancel(context.Background())
	go func(ctx context.Context) {
		s := bufio.NewScanner(os.Stdin)
		for s.Scan() {
			switch cmd := s.Text(); cmd {
			case "":
				continue
			case "...":
				fallthrough
			case "exit":
				fallthrough
			case "quit":
				log.Println("exiting...")
				conn.Send(ctx, func(e *Event, err error) {
					fmt.Println("disconnect reply:", e)
					if err != nil {
						log.Fatal(err)
					}
				}, "...")
				cancel()
				return
			default:
				conn.Api(ctx, func(e *Event, err error) {
					fmt.Println(e.GetTextBody())
				}, cmd)
			}
		}
	}(ctx)
	log.Println("handle events exit:", conn.HandleEvents(ctx))
	conn.Close()
	time.Sleep(2 * time.Second)
}

func okOrDie(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func connErrHandle(err error) {
	if err == nil {
		return
	}
	if wErr, ok := err.(*Error); ok {
		// TODO: retry or something
		switch wErr.Original {
		case io.EOF:
			fallthrough
		case io.EOF:
			fallthrough
		case io.ErrUnexpectedEOF:
			fallthrough
		case io.ErrClosedPipe:
			log.Printf("connection write occuried cause err: %v, please retry.\n", wErr)
		}
	}
}

func (h *Handler) OnConnect(conn *Connection) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	conn.MustSendOK(ctx, "event", "plain", "ALL")

	conn.Api(ctx, func(e *Event, err error) {
		if err != nil {
			log.Fatal(err)
		}
		body := e.GetTextBody()
		if strings.HasPrefix(body, "-ERR ") {
			log.Println("err:", body[5:])
		}
	}, "stat")

	conn.Api(ctx, func(e *Event, err error) {
		log.Println("uuid:", e.GetTextBody())
		h.CallId = e.GetTextBody()
		log.Println("call id:", h.CallId)
	}, "create_uuid")

	h.BgJobId = conn.BgApi(
		ctx,
		connErrHandle,
		"originate",
		"{origination_uuid="+h.CallId+",origination_caller_id_number="+Caller+"}user/"+Callee,
		"&playback(local_stream://moh)",
	)
	log.Println("originate bg job id:", h.BgJobId)
}

func (h *Handler) OnDisconnect(conn *Connection, ev *Event) {
	log.Println("esl disconnected:", ev)
}

func (h *Handler) OnClose(con *Connection) {
	log.Println("esl connection closed")
}

func (h *Handler) OnEvent(ctx context.Context, con *Connection, ev *Event) {
	if ev.Type == EventGeneric {
		log.Printf("fire time: %s\n", ev.Fire.StdTime().Format("2006-01-02 15:04:05"))
	}
	log.Printf("%s - event %s %s %s\n", ev.UId, ev.Name, ev.App, ev.AppData)
	switch ev.Name {
	case BACKGROUND_JOB:
		log.Printf("bg job result:%s\n", ev.GetTextBody())
	case CHANNEL_ANSWER:
		log.Println("call answered, starting moh")
		con.Execute(ctx, nil, "playback", h.CallId, "local_stream://moh")
	case CHANNEL_HANGUP:
		cause := ev.Get("Hangup-Cause")
		log.Printf("call terminated with cause %s", cause)
	}
}
