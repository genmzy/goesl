// Copyright 2022 genmzy. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goesl

import (
	"bytes"
	"fmt"
)

// connection write/read may need a way to judge network error type
// here is the solution to:
//   1. keep original error
//   2. no SendRecv/Api/BgApi method signature modifications
// if you want original error, please use type-switch
type Error struct {
	Original error
	Stack    string
}

func (e *Error) Error() string {
	buf := bytes.Buffer{}
	buf.WriteString(e.Stack)
	buf.WriteString(": ")
	buf.WriteString(e.Original.Error())
	return buf.String()
}

func (e *Error) WithStack(more string) {
	e.Stack = fmt.Sprintf("%s: %s", more, e.Stack)
}
