package main

import (
	"bufio"
	"context"
	"net"
	"os"
	"os/signal"
	"runtime/trace"
	"sync"
	"syscall"
	"time"

	. "github.com/genmzy/goesl"
)

const timeFormat = "2006-01-02 15:04:05"

var authRequest = []byte("Content-Type: auth/request\r\n\r\n")
var authReply = []byte("Content-Type: command/reply\r\nReply-Text: +OK accepted\r\n\r\n")
var heartbeat = []byte("Content-Length: 914\r\nContent-Type: text/event-plain\r\n\r\nEvent-Name: HEARTBEAT\r\nCore-UUID: 3f531e10-0b35-4f5a-abdc-0f6c058095a5\r\nFreeSWITCH-Hostname: sza92131\r\nFreeSWITCH-Switchname: sza92131\r\nFreeSWITCH-IPv4: 10.132.92.131\r\nFreeSWITCH-IPv6: ::1\r\nEvent-Date-Local: 2022-03-23 16:27:45\r\nEvent-Date-GMT: Wed, 23 Mar 2022 08:27:45 GMT\r\nEvent-Date-Timestamp: 1648024065289541\r\nEvent-Calling-File: switch_core.c\r\nEvent-Calling-Function: send_heartbeat\r\nEvent-Calling-Line-Number: 81\r\nEvent-Sequence: 57728710\r\nEvent-Info: System Ready\r\nUp-Time: 0 years, 78 days, 2 hours, 11 minutes, 40 seconds, 201 milliseconds, 301 microseconds\r\nFreeSWITCH-Version: 1.10.5-release+git~20200818T185121Z~25569c1631~64bit\r\nUptime-msec: 6747100201\r\nSession-Count: 39\r\nMax-Sessions: 2000\r\nSession-Per-Sec: 399\r\nSession-Per-Sec-Last: 7\r\nSession-Per-Sec-Max: 169\r\nSession-Per-Sec-FiveMin: 12\r\nSession-Since-Startup: 1862025\r\nSession-Peak-Max: 358\r\nSession-Peak-FiveMin: 73\r\nIdle-CPU: 99.233333\r\n\r\n")
var plainReply = []byte("Content-Type: command/reply\r\nReply-Text: +OK\r\n\r\n")

type Handler struct{}

func (h *Handler) OnConnect(conn *Connection) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	conn.MustSendOK(ctx, "event", "plain", "ALL")
}

func (h *Handler) OnDisconnect(conn *Connection, ev *Event) {
	Noticef("esl disconnected:", ev)
}

func (h *Handler) OnClose(con *Connection) {
	Noticef("esl connection closed")
}

func (h *Handler) OnEvent(ctx context.Context, con *Connection, ev *Event) {
	go func() {
		time.Sleep(2 * time.Millisecond)
		Noticef("%s with fire time: %s\n", ev.Name, ev.Fire.StdTime().Format(timeFormat))
	}()
}

func main() {
	listener, err := net.Listen("tcp", ":8071")
	if err != nil {
		Fatalf(err.Error())
	}
	defer listener.Close()
	go func() {
		sigs := make(chan os.Signal)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGSTOP)
		defer signal.Stop(sigs)

		tracer, err := os.Create("./bin/trace.out")
		if err != nil {
			Fatalf("create file trace.out failed: %v", err)
		}
		defer tracer.Close()
		trace.Start(tracer)
		defer trace.Stop()

		time.Sleep(2 * time.Second)
		Debugf("call start")
		f, err := os.OpenFile(
			"./log/event_generator.log",
			os.O_TRUNC|os.O_WRONLY|os.O_CREATE,
			0644,
		)
		if err != nil {
			Fatalf(err.Error())
		}
		defer f.Close()
		conn, err := Dial(":8071", "ClueCon", &Handler{},
			WithDefaultAutoRedial(),
			WithHeartBeat(20*time.Second),
			WithNetDelay(2*time.Second),
			WithMaxRetries(-1),
			WithLogLevel(LevelDebug),
			WithLogOutput(f),
		)
		if err != nil {
			Fatalf(err.Error())
		}
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			select {
			case <-sigs:
				cancel()
				conn.Close()
			}
		}()
		conn.HandleEvents(ctx)
		cancel()
	}()
	conn, err := listener.Accept()
	if err != nil {
		Fatalf(err.Error())
	}
	defer conn.Close()
	writer := bufio.NewReadWriter(bufio.NewReaderSize(conn, 16*1024), bufio.NewWriter(conn))
	once := sync.Once{}
	once.Do(func() {
		buffer := [512]byte{}
		writer.Write(authRequest)
		writer.Flush()
		// read auth ClueCon\r\n
		writer.Read(buffer[:])
		writer.Write(authReply)
		writer.Flush()
		// read event plain heartbeat
		writer.Read(buffer[:])
		writer.Write(plainReply)
		writer.Flush()
		writer.Read(buffer[:])
		writer.Write(plainReply)
		writer.Flush()
	})
	for {
		time.Sleep(2 * time.Millisecond)
		writer.Write(heartbeat)
		writer.Flush()
	}
}
