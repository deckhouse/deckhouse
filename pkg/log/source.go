// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package log

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

type AddSourceVar struct {
	val atomic.Pointer[bool]
}

func (v *AddSourceVar) Source() *bool {
	return v.val.Load()
}

func (v *AddSourceVar) Set(b bool) {
	v.val.Store(&b)
}

func (v *AddSourceVar) String() string {
	return fmt.Sprintf("AddSourceVar(%t)", *v.Source())
}

// sourceCache keeps the formatted "file:line" of every call site: resolving a PC
// to a file and line is the expensive part of a source, and call sites are fixed.
type sourceCache struct {
	binaryPath string
	format     func(file string, line int) string

	mu sync.RWMutex
	m  map[uintptr]string
}

func newSourceCache() *sourceCache {
	// getting absolute binary path
	binaryPath := filepath.Dir(os.Args[0])
	// if it's go-build temporary folder
	if strings.Contains(binaryPath, "go-build") {
		binaryPath, _ = filepath.Abs("./../")
	}

	format := func(file string, line int) string {
		return file + ":" + strconv.Itoa(line)
	}

	if os.Getenv("IDEA_DEVELOPMENT") != "" {
		format = func(file string, line int) string {
			return " " + file + ":" + strconv.Itoa(line) + " "
		}
	}

	return &sourceCache{
		binaryPath: binaryPath,
		format:     format,
		m:          make(map[uintptr]string),
	}
}

func (c *sourceCache) get(pc uintptr) string {
	if pc == 0 {
		return ""
	}

	c.mu.RLock()
	s, ok := c.m[pc]
	c.mu.RUnlock()

	if ok {
		return s
	}

	frame, _ := runtime.CallersFrames([]uintptr{pc}).Next()

	// trim all folders before project root
	file := strings.TrimPrefix(frame.File, c.binaryPath)
	if file == "" {
		return ""
	}

	// trim first '/'
	s = c.format(file[1:], frame.Line)

	c.mu.Lock()
	c.m[pc] = s
	c.mu.Unlock()

	return s
}
