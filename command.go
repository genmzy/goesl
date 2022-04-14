// Copyright 2022 genmzy. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goesl

import (
	"bytes"
	"context"
	"fmt"
)

type Command struct {
	Sync  bool
	UId   string
	App   string
	Args  string
	Loops uint
}

// Serialize formats (serializes) the command as expected by freeswitch.
func (cmd *Command) Serialize() []byte {
	var buf bytes.Buffer
	buf.WriteString("sendmsg ")
	buf.WriteString(cmd.UId)
	buf.WriteString("\r\ncall-command: execute")
	buf.WriteString("\r\nexecute-app-name: ")
	buf.WriteString(cmd.App)
	buf.WriteString("\r\nexecute-app-args: ")
	buf.WriteString(cmd.Args)
	buf.WriteString("\r\n")

	if cmd.Sync {
		buf.WriteString("event-lock: true\r\n")
	}
	// loops 0 (undefined) are regarded as 1
	if cmd.Loops > 1 {
		buf.WriteString(fmt.Sprintf("loops: %d\r\n", cmd.Loops))
	}
	buf.WriteString("\r\n\r\n")
	return buf.Bytes()
}

// Execute sends Command cmd over Connection and waits for reply.
// Returns the command reply event pointer or an error if any.
// suggest: use RepJustCareError
func (cmd Command) Execute(ctx context.Context, conn *Connection, h RepHandler) {
	srh := sendRepHandler{
		Content: cmd.Serialize(),
		Handler: h,
	}
	select {
	case conn.sendRepHandlers <- srh:
	case <-ctx.Done():
	}
}
