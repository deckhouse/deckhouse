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

package local

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type Command struct {
	used atomic.Bool

	program string
	args    []string
	sudo    bool
	env     map[string]string
	timeout time.Duration

	onStart           func()
	stdoutLineHandler func(line string)
	stderrLineHandler func(line string)

	stdout []byte
	stderr []byte
}

func NewCommand(program string, args ...string) *Command {
	return &Command{
		program: program,
		args:    args,
	}
}

func (c *Command) Run(ctx context.Context) error {
	if !c.used.CompareAndSwap(false, true) {
		return fmt.Errorf("command instance reused")
	}

	cmd, cancel := c.prepareCmd(ctx)
	defer cancel()

	wg := &sync.WaitGroup{}
	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe failed: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe failed: %v", err)
	}
	wg.Add(2)
	go c.scanLines(stdout, stdoutBuf, wg, c.stdoutLineHandler)
	go c.scanLines(stderr, stderrBuf, wg, c.stderrLineHandler)

	if err = cmd.Start(); err != nil {
		return fmt.Errorf("cmd start failed: %v", err)
	}
	if c.onStart != nil {
		c.onStart()
	}

	wg.Wait() // Wait for stdout/stderr reads to complete first
	c.stdout = stdoutBuf.Bytes()
	c.stderr = stderrBuf.Bytes()
	return cmd.Wait()
}

func (c *Command) scanLines(
	stream io.Reader,
	buf *bytes.Buffer,
	wg *sync.WaitGroup,
	handler func(string),
) {
	defer wg.Done()

	scan := bufio.NewScanner(stream)
	for scan.Scan() {
		line := scan.Text()
		buf.WriteString(line)
		if handler != nil {
			handler(line)
		}
	}
	if err := scan.Err(); err != nil {
		log.ErrorF("scan cmd output failed: %v", err)
	}
}

func (c *Command) OnCommandStart(fn func()) {
	c.onStart = fn
}

//func (c *Command) Output(ctx context.Context) ([]byte, []byte, error) {
//	if !c.used.CompareAndSwap(false, true) {
//		return nil, nil, fmt.Errorf("command instance reused")
//	}
//
//	cmd, cancel := c.prepareCmd(ctx)
//	defer cancel()
//
//	var stdout bytes.Buffer
//	cmd.Stdout = &stdout
//
//	if err := cmd.Start(); err != nil {
//		return nil, nil, fmt.Errorf("start %q: %w", c.program, err)
//	}
//	if c.onStart != nil {
//		c.onStart()
//	}
//
//	if err := cmd.Wait(); err != nil {
//		return nil, nil, err
//	}
//	return stdout.Bytes(), nil, nil // stderr is ignored to preserve compatibility with ssh frontend
//}

func (c *Command) Output(ctx context.Context) ([]byte, []byte, error) {
	if !c.used.CompareAndSwap(false, true) {
		return nil, nil, fmt.Errorf("command instance reused")
	}

	cmd, cancel := c.prepareCmd(ctx)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("start %q: %w", c.program, err)
	}
	if c.onStart != nil {
		c.onStart()
	}

	if err := cmd.Wait(); err != nil {
		return nil, stderr.Bytes(), err
	}

	return stdout.Bytes(), nil, nil
}

func (c *Command) CombinedOutput(ctx context.Context) ([]byte, error) {
	if !c.used.CompareAndSwap(false, true) {
		return nil, fmt.Errorf("command instance reused")
	}

	cmd, cancel := c.prepareCmd(ctx)
	defer cancel()

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %q: %w", c.program, err)
	}
	if c.onStart != nil {
		c.onStart()
	}

	if err := cmd.Wait(); err != nil {
		return nil, err
	}

	return output.Bytes(), nil
}

func (c *Command) prepareCmd(ctx context.Context) (*exec.Cmd, context.CancelFunc) {
	bashBuiltins := []string{"bind", "type", "command", "let", "mapfile", "printf", "readarray", "ulimit"}

	program := c.program
	args := c.args
	if c.sudo {
		program = "sudo"
		args = append([]string{c.program}, c.args...)
	} else if slices.Contains(bashBuiltins, program) { // For shell built-in things we need to run bash
		program = "bash"
		args = []string{"-c", strings.Join(append([]string{c.program}, c.args...), " ")}
	}

	ctx, cancel := context.WithCancel(ctx)
	if c.timeout > 0 {
		cancel()
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
	}

	cmd := exec.CommandContext(ctx, program, args...)
	if len(c.env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range c.env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	log.DebugF("Command prepared: %#v", cmd)

	return cmd, cancel
}

func (c *Command) Sudo(_ context.Context) {
	c.sudo = true
}

func (c *Command) WithTimeout(t time.Duration) {
	c.timeout = t
}

func (c *Command) WithEnv(env map[string]string) {
	c.env = env
}

func (c *Command) WithStdoutHandler(h func(line string)) {
	c.stdoutLineHandler = h
}

func (c *Command) WithStderrHandler(h func(line string)) {
	c.stderrLineHandler = h
}

func (c *Command) StdoutBytes() []byte {
	return c.stdout
}

func (c *Command) StderrBytes() []byte {
	return c.stderr
}

// The rest are no-ops for local execution

func (c *Command) Cmd(_ context.Context)   {}
func (c *Command) WithSSHArgs(_ ...string) {}
