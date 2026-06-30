// Copyright 2026 Flant JSC
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

package logger

import (
	"io"
	"strings"
)

// plainSink is a lineSink that never touches pterm. It writes the rendered lines (milestones,
// warns, framed process boxes, banner, connection string) straight to the underlying writer — the
// logboek-style dump. It is the backend when the writer is not a real terminal (a bytes.Buffer in
// tests, or a redirected/piped stdout) or for the non-interactive -v dump, so output never emits
// ANSI control sequences and no pinned block is started. It has no progress bar: the renderer holds
// bar == nil for this backend, so the bar-cluster methods need not exist here.
type plainSink struct {
	w io.Writer
}

// newPlainSink returns a lineSink that writes plain lines to w.
func newPlainSink(w io.Writer) lineSink {
	return &plainSink{w: w}
}

func (p *plainSink) printf(s string) {
	if p.w == nil {
		return
	}
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	_, _ = io.WriteString(p.w, s)
}

func (p *plainSink) Milestone(status, text string) { p.printf(status + " " + text) }
func (p *plainSink) Warn(line string)              { p.printf(line) }
func (p *plainSink) Log(line string)               { p.printf(line) }

func (p *plainSink) SetBanner(lines []string) {
	for _, l := range lines {
		p.printf(l)
	}
}

func (p *plainSink) SetConnString(s string) { p.printf(s) }
