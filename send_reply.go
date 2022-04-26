// Copyright 2022 genmzy. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goesl

import (
	"fmt"
	"strings"
)

type RepHandler func(*Event, error)
type ConnErrHandler func(error)
type RepErrHandler func(error)

type cmdReplyHandler struct {
	cmd []byte
	rh  RepHandler
}

// ------------------------------ helper for reply ------------------------------- //

type RepJustCareError struct {
	CHandle ConnErrHandler
	RHandle RepErrHandler
}

func (r *RepJustCareError) RepHandle(ev *Event, err error) {
	if err != nil && r.CHandle != nil {
		r.CHandle(err)
	}
	if r.RHandle == nil {
		return
	}
	var reply string
	switch ev.Type {
	case EventCommandReply:
		reply = ev.Get("Reply-Text")
	case EventApiResponse:
		reply = ev.GetTextBody()
	}
	if strings.HasPrefix(reply, "-ERR ") {
		err = fmt.Errorf(reply[5:])
		r.RHandle(err)
	}
}

type RepBgUuid struct {
	CHandle ConnErrHandler
}

// only -ERR permission denied when userauth, not support now
func (r *RepBgUuid) RepHandle(ev *Event, err error) {
	// ignore event receive
	if err != nil && r.CHandle != nil {
		r.CHandle(err)
	}
}
