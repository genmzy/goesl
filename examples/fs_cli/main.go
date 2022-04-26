package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
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
	conn, err := Dial(
		"127.0.0.1:8021",
		// "10.172.49.21:8021",
		"ClueCon",
		handler,
		WithDefaultAutoRedial(),
		WithHeartBeat(20*time.Second),
		WithNetDelay(2*time.Second),
		WithMaxRetries(-1),
		WithLogLevel(LevelDebug),
	)
	if err != nil {
		log.Fatal("connecting to freeswitch: ", err)
	}
	// close twice is allowed
	defer conn.Close()

	go func() {
		sigs := make(chan os.Signal)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sigs)
		select {
		case <-sigs:
			panic("show all goroutines")
		}
	}()

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
				cancel()
				// close twice is allowed
				conn.Close()
				return
			default:
				log.Println("send commands: ", cmd)
				conn.Api(ctx, func(e *Event, err error) {
					fmt.Println(e.GetTextBody())
				}, cmd)
			}
		}
	}(ctx)

	err = conn.HandleEvents(ctx)
	if errors.Is(err, net.ErrClosed) || errors.Is(err, context.Canceled) {
		log.Println("process exiting...")
	} else {
		log.Fatalf("exiting with error: %v", err)
	}
}

func okOrDie(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func errHandle(err error) {
	return
}

func (h *Handler) OnConnect(conn *Connection) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	conn.MustSendOK(
		ctx, "event", "plain",
		CHANNEL_ANSWER.String(), CHANNEL_HANGUP.String(), BACKGROUND_JOB.String(),
	)

	// this will console log `err: stat Command not found!`
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
		nil,
		"originate",
		"{origination_uuid="+h.CallId+",origination_caller_id_number="+Caller+"}user/"+Callee,
		"&echo()",
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
		log.Printf("bg job result: %s\n", ev.GetTextBody())
	case CHANNEL_ANSWER:
		log.Println("call answered, starting moh")
		con.Execute(ctx, nil, "playback", h.CallId, "local_stream://moh")
	case CHANNEL_HANGUP:
		cause := ev.Get("Hangup-Cause")
		log.Printf("call terminated with cause %s", cause)
	}
}
