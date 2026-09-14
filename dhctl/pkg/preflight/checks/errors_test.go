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

package checks

import (
	"errors"
	"fmt"
	"os/exec"
	"testing"
)

// cryptoExitError stands in for x/crypto/ssh's *ExitError, which is what the default gossh
// backend returns. It reports its status through ExitStatus(), not ExitCode() — which is why
// every branch matching on *exec.ExitError was unreachable on the backend dhctl actually uses.
type cryptoExitError struct{ status int }

func (e *cryptoExitError) Error() string {
	return fmt.Sprintf("Process exited with status %d", e.status)
}
func (e *cryptoExitError) ExitStatus() int { return e.status }

func TestExitStatusReadsBothBackends(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantOK     bool
	}{
		{"gossh: x/crypto ExitStatus()", &cryptoExitError{status: 7}, 7, true},
		{"gossh, wrapped", fmt.Errorf("running the script: %w", &cryptoExitError{status: 1}), 1, true},
		{"clissh: os/exec ExitCode()", &exec.ExitError{}, -1, true},
		{"a transport error carries no status", errors.New("i/o timeout"), 0, false},
		{"no error", nil, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, ok := exitStatus(tt.err)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && status != tt.wantStatus {
				t.Errorf("status = %d, want %d", status, tt.wantStatus)
			}
		})
	}
}

// TestScriptFailurePrefersWhatTheScriptSaid: the script's own diagnostics are the only text in
// play written for the operator, so they outrank both the exit status and the transport error.
func TestScriptFailurePrefersWhatTheScriptSaid(t *testing.T) {
	tests := []struct {
		name string
		out  []byte
		err  error
		want string
	}{
		{
			name: "the script explained itself",
			out:  []byte("port 6443 is already in use\n"),
			err:  &cryptoExitError{status: 1},
			want: "port 6443 is already in use on the installer host",
		},
		{
			name: "it only exited non-zero",
			out:  nil,
			err:  &cryptoExitError{status: 3},
			want: "script exited with status 3 on the installer host",
		},
		{
			name: "it never ran",
			out:  nil,
			err:  errors.New("dial tcp 10.0.0.5:22: i/o timeout"),
			want: "cannot check ports on the installer host: dial tcp 10.0.0.5:22: i/o timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// A nil node interface is the local case: no SSH connection behind it.
			got := scriptFailure("check ports", nil, tt.out, tt.err)
			if got.Error() != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestScriptFailureKeepsTheCauseReachable: the transport error has to survive the wording, or a
// caller can no longer tell a timeout from a refused connection.
func TestScriptFailureKeepsTheCauseReachable(t *testing.T) {
	cause := errors.New("i/o timeout")
	if err := scriptFailure("check ports", nil, nil, cause); !errors.Is(err, cause) {
		t.Errorf("the cause must stay reachable, got %v", err)
	}
}

// TestHostPhraseWithoutAnSSHConnection names the machine honestly rather than implying a node.
func TestHostPhraseWithoutAnSSHConnection(t *testing.T) {
	if got := hostPhrase(nil); got != "the installer host" {
		t.Errorf("got %q, want %q", got, "the installer host")
	}
}
