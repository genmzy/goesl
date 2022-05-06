package goesl

import (
	"io"
	"time"
)

type Options struct {
	autoRedial   bool
	nextDialWait func([]time.Duration) time.Duration
	maxRetries   int

	dialTimeout time.Duration
	heartbeat   time.Duration // event heartbeat time
	netDelay    time.Duration // event heartbeat delay, default 2 seconds

	sendReplyCap int

	logger Logger
}

func (o *Options) apply(opts []Option) {
	for _, op := range opts {
		op.f(o)
	}
}

func newOptions(opts []Option) *Options {
	o := &Options{
		autoRedial:   false,
		nextDialWait: nil,
		maxRetries:   -1,
		// timeouts
		dialTimeout: 3 * time.Second,
		heartbeat:   20 * time.Second,
		netDelay:    2 * time.Second,
		// sender messages
		sendReplyCap: 20,
		// debug logger
		logger: logger,
	}
	o.apply(opts)
	return o
}

type Option struct {
	f func(*Options)
}

// need to pay attention that the `strategy` should handle that time.Duration is empty
func WithAutoRedial(strategy func([]time.Duration) time.Duration) Option {
	return Option{
		f: func(o *Options) {
			o.autoRedial = true
			o.nextDialWait = strategy
		},
	}
}

// 1, 2, 4, 8, 16, 16, ...
func defaultRedialWait(old []time.Duration) (next time.Duration) {
	if len(old) == 0 {
		return 1 * time.Second
	}
	curSec := int(old[len(old)-1] / time.Second)
	if curSec < 16 {
		return time.Duration(curSec*2) * time.Second
	}
	return 16 * time.Second
}

func WithDefaultAutoRedial() Option {
	return WithAutoRedial(defaultRedialWait)
}

func WithLogger(logger Logger) Option {
	return Option{
		f: func(o *Options) { o.logger = logger },
	}
}

// if n is -1, always retry
func WithMaxRetries(n int) Option {
	return Option{
		f: func(o *Options) {
			if n <= 0 || n > 100 {
				return
			}
			o.maxRetries = n
		},
	}
}

func WithDialTimeout(t time.Duration) Option {
	return Option{
		f: func(o *Options) { o.dialTimeout = t },
	}
}

// Set heartbeat interval time duration
// only take effect when set with `WithAutoRedial`
// event receiver regards connection lost when `heartbeat_interval + net_delay > time_wait`
func WithHeartBeat(t time.Duration) Option {
	return Option{
		f: func(o *Options) { o.heartbeat = t },
	}
}

// Set max network delay time duration
// used as connection write timeout, send-reply ticker timeout
// suggest range:       1*time.Second <= t <= 5*time.Second
// valid range: 100*time.Milliseconds <= t <= 10*time.Second
func WithNetDelay(t time.Duration) Option {
	return Option{
		f: func(o *Options) {
			if t.Milliseconds() >= 100 && t.Seconds() <= 10 {
				o.netDelay = t
			}
		},
	}
}

// Sender channel capacity
func WithSendReplyCap(n int) Option {
	return Option{
		f: func(o *Options) {
			if n <= 0 || n > 500 {
				return
			}
			o.sendReplyCap = n
		},
	}
}

// Set logger level, ONLY set level of internal logger
// if a user defined logger, do nothing
func WithLogLevel(lv Level) Option {
	return Option{
		f: func(o *Options) {
			if o.logger != logger {
				return
			}
			logger.level = lv
		},
	}
}

// Set logger output file, only set output file of set internal logger
// if a user defined logger, do nothing
func WithLogOutput(w io.Writer) Option {
	return Option{
		f: func(o *Options) {
			if o.logger != logger {
				return
			}
			logger.SetOutput(w)
		},
	}
}

func WithLogPrefix(s string) Option {
	return Option{
		f: func(o *Options) {
			if o.logger != logger {
				return
			}
			logger.SetPrefix(s)
		},
	}
}
