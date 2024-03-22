package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/genmzy/goesl"
)

type Handler struct{}

var (
	host    = flag.String("H", "127.0.0.1", "Specify the host of freeswitch")
	port    = flag.Int("P", 8021, "Specify the port of freeswitch")
	passwd  = flag.String("p", "ClueCon", "Specify the default password of freeswitch")
	level   = flag.String("l", "debug", "Specify the log level of freeswitch")
	logPath = flag.String("log_file", "", "Specify the log path of fs_cli self")
)

func main() {
	flag.Parse()

	go signalCatch()

	opts := make([]goesl.Option, 0)
	opts = append(opts,
		goesl.WithDefaultAutoRedial(),
		goesl.WithHeartBeat(20*time.Second),
		goesl.WithNetDelay(2*time.Second),
		goesl.WithMaxRetries(-1),
		goesl.WithLogLevel(goesl.LevelDebug),
		goesl.WithLogPrefix("<fs_cli>"),
	)

	if *logPath != "" {
		logFile, err := os.OpenFile(*logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0755)
		if err != nil {
			goesl.Warnf("error when open log file %s: %v, use stdout now ...", *logPath, err)
		} else {
			opts = append(opts, goesl.WithLogOutput(logFile))
			defer logFile.Close()
		}
	}

	handler := &Handler{}
	conn, err := goesl.Dial(fmt.Sprintf("%s:%d", *host, *port), *passwd, handler, opts...)
	if err != nil {
		goesl.Fatalf("connecting to freeswitch: %v", err)
	}
	// close twice is allowed
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go messageSend(ctx, conn, cancel)

	err = conn.HandleEvents(ctx)
	if err == nil || errors.Is(err, net.ErrClosed) || errors.Is(err, context.Canceled) {
		goesl.Noticef("process exiting...")
	} else {
		goesl.Errorf("exiting with error: %v", err)
	}
}

type SofiaEntry struct {
	Name   string
	Type   string // gateway or profile
	Url    string
	Status string
	Extra  string
}

func (se SofiaEntry) String() string {
	return fmt.Sprintf(`{"name":"%s","type":"%s","url":"%s","status":"%s","extra":"%s"}`,
		se.Name, se.Type, se.Url, se.Status, se.Extra)
}

// return sofia entry groups
func SofiaStatus(raw []byte) (entries []*SofiaEntry) {
	s := bufio.NewScanner(bytes.NewBuffer(raw))
	start := false
	for s.Scan() {
		t := s.Text()
		if t[0] == '=' { // ====================
			if start { // end line
				break
			} else { // start line
				start = true
				continue
			}
		}
		if !start {
			continue
		}
		// content line now
		t = strings.TrimSpace(t)
		reg := regexp.MustCompile(`\s+`)
		sli := reg.Split(t, -1)
		entry := &SofiaEntry{Name: sli[0], Type: sli[1], Url: sli[2], Status: sli[3]}
		if len(sli) > 4 {
			entry.Extra = sli[4]
		}
		entries = append(entries, entry)
	}
	return
}

func (h *Handler) OnConnect(conn *goesl.Connection) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	// conn.Plain(ctx, []ev_name.EventName{ev_name.BACKGROUND_JOB, ev_name.API}, nil)
	conn.Fslog(ctx, goesl.FslogString2Level(*level))

	die := func(s string, err error) string {
		if err != nil {
			conn.Close()
			goesl.Fatalf("api: %v", err)
		}
		return s
	}

	core := die(conn.Api(ctx, "global_getvar", "core_uuid"))
	raw := die(conn.Api(ctx, "sofia", "status"))
	entries := SofiaStatus([]byte(raw))
	goesl.Noticef("connect and get freeswitch (core: %s with sofia entries: %v)", core, entries)
}

func (h *Handler) OnDisconnect(conn *goesl.Connection, ev goesl.Event) {
	goesl.Noticef("esl disconnected: %v", ev)
}

func (h *Handler) OnClose(con *goesl.Connection) {
	goesl.Noticef("esl connection closed")
}

func (h *Handler) OnEvent(ctx context.Context, conn *goesl.Connection, e goesl.Event) {
	fmt.Print("\nRECV EVENT\n", e.EventContent())
}

var colorFormats = map[goesl.FslogLevel]string{
	goesl.FslogLevel_DEBUG:   "\033[0;33m%s\033[0m\n",
	goesl.FslogLevel_INFO:    "\033[0;32m%s\033[0m\n",
	goesl.FslogLevel_NOTICE:  "\033[0;36m%s\033[0m\n",
	goesl.FslogLevel_WARNING: "\033[0;35m%s\033[0m\n",
	goesl.FslogLevel_ERROR:   "\033[0;31m%s\033[0m\n",
	goesl.FslogLevel_CRIT:    "\033[0;31m%s\033[0m\n",
	goesl.FslogLevel_ALERT:   "\033[0;31m%s\033[0m\n",
}

func getColorFormat(lv goesl.FslogLevel) string {
	if format, ok := colorFormats[lv]; ok {
		return format
	} else {
		return "%s\n\n"
	}
}

func (h *Handler) OnFslog(ctx context.Context, conn *goesl.Connection, fslog goesl.Fslog) {
	var content string
	if fslog.UserData != "" {
		content = fmt.Sprintf("%s %s", fslog.UserData, fslog.Content)
	} else {
		content = fslog.Content
	}
	fmt.Printf(getColorFormat(fslog.Level), content)
}

func send(ctx context.Context, conn *goesl.Connection, cmd string) {
	check := func(res string, err error) {
		if err != nil {
			goesl.Errorf("api %s error: %v", cmd, err)
			return
		}
		fmt.Println(res)
	}

	if cmd[0] == '/' {
		check(conn.Send(ctx, cmd[1:]))
		return
	}
	if strings.Split(cmd, " ")[0] == "originate" {
		goesl.Noticef("now use bapi originate instead of originate")
		check(conn.BgApi(ctx, cmd))
		return
	}
	check(conn.Api(ctx, cmd))
}

func messageSend(ctx context.Context, conn *goesl.Connection, cancel func()) {
	s := bufio.NewScanner(os.Stdin)
	host, err := conn.Api(ctx, "global_getvar", "hostname")
	if err != nil {
		goesl.Errorf("api global_getvar hostname send: %v", err)
		return
	}
	cliPrefix := fmt.Sprintf("freeswitch/%s> ", host)
	fmt.Print(cliPrefix)
	for s.Scan() {
		fmt.Print(cliPrefix)
		switch cmd := s.Text(); cmd {
		case "":
		case "...", "/exit", "/bye", "/quit":
			cancel()
			// close twice is allowed
			conn.Close()
			return
		default:
			send(ctx, conn, cmd)
		}
	}
}

func signalCatch() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)
	<-sigs
	// need to set environment `GOTRACEBACK=1`
	panic("show all goroutines")
}
