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
	"context"
	"fmt"
	"strings"
	"time"

	libcon "github.com/deckhouse/lib-connection/pkg"
)

// A node described by what its commands answer.
//
// The mocks in checks/mocks are call recorders: a test has to declare every command with its exact
// arguments, and a check that probes one more thing fails with an unexpected-call panic rather
// than with what it found. That made the on-node checks expensive to test and, in practice,
// mostly untested.
//
// This fake answers instead. A test says what `df -Pk /var/lib` prints and what `command -v curl`
// exits with; everything else gets the default answer, which is the one a check should treat as
// "not there". What is asked can still be asserted afterwards, through ran().
//
// The exit status matters as much as the output. A non-zero status is delivered as a
// *fakeSSHExitError, which reports itself through ExitStatus() — the shape x/crypto produces and
// the default gossh backend returns. os/exec's *exec.ExitError, which reports through ExitCode(),
// is what the checks used to match on, which is why those branches were unreachable in production
// and why the tests that used them proved nothing.
type fakeNode struct {
	answers map[string]fakeAnswer
	scripts map[string]fakeAnswer
	missing fakeAnswer
	asked   []string

	// current is the command the setters below apply to, named by the last on/onScript.
	current         string
	currentIsScript bool
}

type fakeAnswer struct {
	stdout string
	stderr string
	// status is the exit status the command ends with. Zero is success.
	status int
	// transportErr stands for a failure to run the command at all — a dropped connection, a
	// closed session — which carries no exit status.
	transportErr error
}

func newFakeNode() *fakeNode {
	return &fakeNode{
		answers: map[string]fakeAnswer{},
		scripts: map[string]fakeAnswer{},
		// An undeclared command is one the node does not have: `command -v` exits 127 for a
		// binary that is not on PATH, and that is the answer nearly every probe is written
		// against.
		missing: fakeAnswer{status: 127},
	}
}

// on declares the answer to one command line, written the way the check builds it. The setters
// that follow apply to it, and chain:
//
//	newFakeNode().
//		on("command -v sudo").prints("/usr/bin/sudo").
//		on("sudo -n true").exits(1).stderr("sudo: a password is required")
func (n *fakeNode) on(commandLine string) *fakeNode {
	n.current, n.currentIsScript = commandLine, false
	if _, declared := n.answers[commandLine]; !declared {
		n.answers[commandLine] = fakeAnswer{}
	}
	return n
}

// onScript declares the answer to an uploaded script, matched on the tail of its path so a test
// does not have to know the temporary directory it was rendered into.
func (n *fakeNode) onScript(pathSuffix string) *fakeNode {
	n.current, n.currentIsScript = pathSuffix, true
	if _, declared := n.scripts[pathSuffix]; !declared {
		n.scripts[pathSuffix] = fakeAnswer{}
	}
	return n
}

// ran returns the command lines the check actually ran, in order.
func (n *fakeNode) ran() []string {
	return append([]string(nil), n.asked...)
}

// amend applies a change to the answer the last on/onScript named.
func (n *fakeNode) amend(change func(*fakeAnswer)) *fakeNode {
	if n.current == "" {
		panic("fakeNode: a setter was called before on() or onScript() named a command")
	}
	store := n.answers
	if n.currentIsScript {
		store = n.scripts
	}
	answer := store[n.current]
	change(&answer)
	store[n.current] = answer
	return n
}

func (n *fakeNode) succeeds() *fakeNode { return n.amend(func(*fakeAnswer) {}) }

func (n *fakeNode) prints(out string) *fakeNode {
	return n.amend(func(a *fakeAnswer) { a.stdout = out })
}

func (n *fakeNode) exits(status int) *fakeNode {
	return n.amend(func(a *fakeAnswer) { a.status = status })
}

func (n *fakeNode) stderr(text string) *fakeNode {
	return n.amend(func(a *fakeAnswer) { a.stderr = text })
}

// printsAndExits is for a command that says what is wrong and still fails, which is how a script
// reports the state of the node.
func (n *fakeNode) printsAndExits(out string, status int) *fakeNode {
	return n.prints(out).exits(status)
}

// fails stands for a failure to run the command at all — a dropped connection, a closed session —
// which carries no exit status.
func (n *fakeNode) fails(err error) *fakeNode {
	return n.amend(func(a *fakeAnswer) { a.transportErr = err })
}

func (n *fakeNode) answerFor(commandLine string) fakeAnswer {
	n.asked = append(n.asked, commandLine)
	if answer, ok := n.answers[commandLine]; ok {
		return answer
	}
	return n.missing
}

func (n *fakeNode) answerForScript(path string) fakeAnswer {
	n.asked = append(n.asked, path)
	for suffix, answer := range n.scripts {
		if strings.HasSuffix(path, suffix) {
			return answer
		}
	}
	return fakeAnswer{}
}

// err turns the declared answer into the error the backend would produce.
func (a fakeAnswer) err() error {
	switch {
	case a.transportErr != nil:
		return a.transportErr
	case a.status != 0:
		return &fakeSSHExitError{status: a.status}
	default:
		return nil
	}
}

// fakeSSHExitError has the shape of x/crypto/ssh.ExitError: the status is behind ExitStatus(),
// not ExitCode().
type fakeSSHExitError struct{ status int }

func (e *fakeSSHExitError) Error() string {
	return fmt.Sprintf("Process exited with status %d", e.status)
}
func (e *fakeSSHExitError) ExitStatus() int { return e.status }

// libcon.Interface

func (n *fakeNode) Command(name string, args ...string) libcon.Command {
	return &fakeCommand{node: n, line: strings.TrimSpace(name + " " + strings.Join(args, " "))}
}

func (n *fakeNode) UploadScript(scriptPath string, _ ...string) libcon.Script {
	return &fakeScript{node: n, path: scriptPath}
}

func (n *fakeNode) File() libcon.File { return &fakeFile{} }

// libcon.Command

type fakeCommand struct {
	node   *fakeNode
	line   string
	answer fakeAnswer
}

func (c *fakeCommand) run() error {
	c.answer = c.node.answerFor(c.line)
	return c.answer.err()
}

func (c *fakeCommand) Run(context.Context) error { return c.run() }

func (c *fakeCommand) Output(context.Context) ([]byte, []byte, error) {
	err := c.run()
	return []byte(c.answer.stdout), []byte(c.answer.stderr), err
}

func (c *fakeCommand) CombinedOutput(context.Context) ([]byte, error) {
	err := c.run()
	return []byte(c.answer.stdout + c.answer.stderr), err
}

func (c *fakeCommand) StdoutBytes() []byte { return []byte(c.answer.stdout) }
func (c *fakeCommand) StderrBytes() []byte { return []byte(c.answer.stderr) }

func (c *fakeCommand) Cmd(context.Context)  {}
func (c *fakeCommand) Sudo(context.Context) {}

func (c *fakeCommand) OnCommandStart(func())          {}
func (c *fakeCommand) WithEnv(map[string]string)      {}
func (c *fakeCommand) WithTimeout(time.Duration)      {}
func (c *fakeCommand) WithStdoutHandler(func(string)) {}
func (c *fakeCommand) WithStderrHandler(func(string)) {}
func (c *fakeCommand) WithSSHArgs(...string)          {}

// libcon.Script

type fakeScript struct {
	node *fakeNode
	path string
}

func (s *fakeScript) Execute(context.Context) ([]byte, error) {
	answer := s.node.answerForScript(s.path)
	return []byte(answer.stdout), answer.err()
}

func (s *fakeScript) ExecuteBundle(context.Context, string, string) ([]byte, error) {
	answer := s.node.answerForScript(s.path)
	return []byte(answer.stdout), answer.err()
}

func (s *fakeScript) Sudo()                                   {}
func (s *fakeScript) WithStdoutHandler(func(string))          {}
func (s *fakeScript) WithTimeout(time.Duration)               {}
func (s *fakeScript) WithEnvs(map[string]string)              {}
func (s *fakeScript) WithCleanupAfterExec(bool)               {}
func (s *fakeScript) WithNoLogStepOutOnError(bool)            {}
func (s *fakeScript) WithExecuteUploadDir(string)             {}
func (s *fakeScript) WithBundlerOpts(...libcon.BundlerOption) {}

// libcon.File

type fakeFile struct{}

func (*fakeFile) Upload(context.Context, string, string) error      { return nil }
func (*fakeFile) Download(context.Context, string, string) error    { return nil }
func (*fakeFile) UploadBytes(context.Context, []byte, string) error { return nil }
func (*fakeFile) DownloadBytes(context.Context, string) ([]byte, error) {
	return nil, nil
}
