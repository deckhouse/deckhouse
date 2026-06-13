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

import "io"

// plainProgressUI is a progressUI that never touches pterm. It writes ordinary lines straight
// to the underlying writer and treats every progress/spinner method as a no-op. It is used as
// the TTY sink's UI when the writer is not a real terminal (e.g. a bytes.Buffer in tests, or a
// redirected/piped stdout), so non-terminal output never emits ANSI escape sequences and the
// pterm MultiPrinter is never started.
type plainProgressUI struct {
	w io.Writer
}

// newPlainProgressUI returns a progressUI that writes plain lines to w and no-ops everything else.
func newPlainProgressUI(w io.Writer) progressUI {
	return &plainProgressUI{w: w}
}

func (p *plainProgressUI) Start(string)                {}
func (p *plainProgressUI) SetProgress(float64, string) {}
func (p *plainProgressUI) SetAction(string)            {}

func (p *plainProgressUI) WriteLine(s string) {
	if p.w == nil {
		return
	}
	_, _ = io.WriteString(p.w, s)
}

func (p *plainProgressUI) Finish() {}
func (p *plainProgressUI) Pause()  {}
func (p *plainProgressUI) Resume() {}
func (p *plainProgressUI) Resize() {}
