// Copyright 2022 genmzy. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goesl

type EslLogger interface {
	Printf(fmt string, v ...interface{})
	Fatal(v ...interface{})
}
