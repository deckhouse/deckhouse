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
	"bytes"
	"strings"
	"testing"
)

func TestNewRootWritesToFileSink(t *testing.T) {
	var file bytes.Buffer
	l := NewRoot(Options{FileWriter: &file, Debug: true})
	l.Info("root file record")
	if !strings.Contains(file.String(), "root file record") {
		t.Fatalf("file sink missing record: %q", file.String())
	}
}

func TestNewRootFileAlwaysCapturesDebug(t *testing.T) {
	// The debug file is always complete regardless of the Debug flag (which only drives terminal
	// verbosity now).
	var file bytes.Buffer
	l := NewRoot(Options{FileWriter: &file, Debug: false})
	l.Debug("debug detail")
	l.Info("info line")
	if !strings.Contains(file.String(), "debug detail") || !strings.Contains(file.String(), "info line") {
		t.Fatalf("file must capture debug + info regardless of flag: %q", file.String())
	}
}

func TestNewRootNormalModeTTYGating(t *testing.T) {
	// Debug=false (normal): terminal shows only ShowInCompacted()-tagged records; the file gets both.
	var file, tty bytes.Buffer
	l := NewRoot(Options{FileWriter: &file, TTYWriter: &tty, IsTTY: true, Debug: false})

	l.Info("plain") // file only
	l.Info("shown", ShowInCompacted())

	if !strings.Contains(file.String(), "plain") || !strings.Contains(file.String(), "shown") {
		t.Fatalf("file must contain both records: %q", file.String())
	}
	if strings.Contains(tty.String(), "plain") {
		t.Fatalf("untagged record leaked to tty in normal mode: %q", tty.String())
	}
	if !strings.Contains(tty.String(), "shown") {
		t.Fatalf("tagged record missing from tty: %q", tty.String())
	}
}

func TestNewRootVerboseShowsEverythingOnTTY(t *testing.T) {
	// Verbose=true (-v): terminal shows untagged Info records too.
	var file, tty bytes.Buffer
	l := NewRoot(Options{FileWriter: &file, TTYWriter: &tty, IsTTY: true, Verbose: true})
	l.Info("plain untagged")
	if !strings.Contains(tty.String(), "plain untagged") {
		t.Fatalf("verbose tty must show untagged record: %q", tty.String())
	}
}

func TestNewRootVerboseAloneHidesDebugOnTTY(t *testing.T) {
	// -v shows all Info+ on the terminal but NOT DEBUG; DEBUG needs Debug (DHCTL_DEBUG). File keeps both.
	var file, tty bytes.Buffer
	l := NewRoot(Options{FileWriter: &file, TTYWriter: &tty, IsTTY: true, Verbose: true, Debug: false})
	l.Debug("debug detail")
	l.Info("info detail")
	if strings.Contains(tty.String(), "debug detail") {
		t.Fatalf("DEBUG must not reach tty without debug mode: %q", tty.String())
	}
	if !strings.Contains(tty.String(), "info detail") {
		t.Fatalf("verbose tty must show info: %q", tty.String())
	}
	if !strings.Contains(file.String(), "debug detail") {
		t.Fatalf("file must keep DEBUG: %q", file.String())
	}
}

func TestNewRootDebugModeShowsDebugOnTTY(t *testing.T) {
	// Debug=true (DHCTL_DEBUG): DEBUG records reach the terminal. Implies verbose in production wiring.
	var file, tty bytes.Buffer
	l := NewRoot(Options{FileWriter: &file, TTYWriter: &tty, IsTTY: true, Verbose: true, Debug: true})
	l.Debug("debug detail")
	if !strings.Contains(tty.String(), "debug detail") {
		t.Fatalf("debug mode tty must show DEBUG: %q", tty.String())
	}
}

func TestNewRootNoTTYWriterMeansFileOnly(t *testing.T) {
	var file bytes.Buffer
	l := NewRoot(Options{FileWriter: &file, Verbose: true})
	l.Info("only file", ShowInCompacted())
	if !strings.Contains(file.String(), "only file") {
		t.Fatalf("file missing record: %q", file.String())
	}
}
