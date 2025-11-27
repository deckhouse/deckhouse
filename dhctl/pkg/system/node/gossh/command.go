package gossh

// Copyright 2025 Flant JSC
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

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	ssh "github.com/deckhouse/lib-gossh"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/process"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

type SSHCommand struct {
	sshClient *Client
	session   *ssh.Session

	Name string
	Args []string
	Env  []string

	SSHArgs []string

	stdoutPipeFile io.Reader
	stderrPipeFile io.Reader
	StdoutSplitter bufio.SplitFunc

	StdinPipe bool
	Stdin     io.WriteCloser

	Matchers     []*process.ByteSequenceMatcher
	MatchHandler func(pattern string) string

	onCommandStart func()
	stderrHandler  func(string)
	stdoutHandler  func(string)

	WaitHandler func(err error)

	out      *bytes.Buffer
	err      *bytes.Buffer
	combined *singleWriter

	OutBytes bytes.Buffer
	ErrBytes bytes.Buffer

	stop   bool
	waitCh chan struct{}
	stopCh chan struct{}

	lockWaitError sync.RWMutex
	waitError     error
	killError     error

	cmd     string
	timeout time.Duration

	ctx       context.Context
	Cancel    func() error
	ctxResult <-chan error
	wg        sync.WaitGroup
}

func NewSSHCommand(client *Client, name string, arg ...string) *SSHCommand {
	args := make([]string, len(arg))
	copy(args, arg)
	cmd := name + " "
	for i := range args {
		if !strings.HasPrefix(args[i], `"`) &&
			!strings.HasSuffix(args[i], `"`) &&
			strings.Contains(args[i], " ") {
			args[i] = strconv.Quote(args[i])
		}
	}

	var session *ssh.Session
	var err error

	err = retry.NewSilentLoop("Establish new session", 10, 5*time.Second).Run(func() error {
		session, err = client.sshClient.NewSession()
		return err
	})

	return &SSHCommand{
		// Executor: process.NewDefaultExecutor(sess.Run(cmd)),
		sshClient: client,
		session:   session,
		Name:      name,
		Args:      args,
		Env:       os.Environ(),
		cmd:       cmd,
	}
}

func (c *SSHCommand) WithSSHArgs(args ...string) {
	c.SSHArgs = args
}

func (c *SSHCommand) OnCommandStart(fn func()) {
	c.onCommandStart = fn
}

func (c *SSHCommand) Start() error {
	// setup stream handlers
	command := c.cmd + " " + strings.Join(c.Args, " ")
	log.DebugF("executor: start '%s'\n", command)
	if c.session == nil {
		return fmt.Errorf("ssh session not started")
	}

	err := c.SetupStreamHandlers()
	if err != nil {
		log.DebugF("could not set up stream handlers: %s\n", err)
		return err
	}

	err = c.start()
	if err != nil {
		log.DebugF("could not start\n")
		return err
	}

	// if c.WaitHandler != nil {
	if c.WaitHandler != nil || c.timeout > 0 {
		c.ProcessWait()
	} else {
		err = c.wait()
		if err != nil {
			return err
		}
	}

	log.DebugF("Register stoppable: '%s'\n", command)
	c.sshClient.RegisterSession(c.session)

	return nil
}

func (c *SSHCommand) start() error {
	if c.ctx != nil {
		select {
		case <-c.ctx.Done():
			return c.ctx.Err()
		default:
		}
	}

	if c.Cancel != nil && c.ctx != nil && c.ctx.Done() != nil {
		resultc := make(chan error)
		c.ctxResult = resultc
		go c.watchCtx(resultc)
	}

	command := c.cmd + " " + strings.Join(c.Args, " ")

	return c.session.Start(command)
}

func (c *SSHCommand) watchCtx(resultc chan<- error) {
	<-c.ctx.Done()

	var err error
	if c.Cancel != nil {
		if interruptErr := c.Cancel(); interruptErr == nil {
			// We appear to have successfully interrupted the command, so any
			// program behavior from this point may be due to ctx even if the
			// command exits with code 0.
			err = c.ctx.Err()
		} else if errors.Is(interruptErr, os.ErrProcessDone) {
			// The process already finished: we just didn't notice it yet.
			// (Perhaps c.Wait hadn't been called, or perhaps it happened to race with
			// c.ctx being canceled.) Don't inject a needless error.
		} else {
			err = interruptErr
		}
	}

	resultc <- err
}

func (c *SSHCommand) wait() error {
	waitCh := make(chan (error))

	go func() {
		waitCh <- c.session.Wait()
	}()

	select {
	case err := <-c.ctxResult:
		if c.ctxResult != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
		}
	case err := <-waitCh:
		if err != nil {
			return err
		}

	}
	return nil
}

func (c *SSHCommand) ProcessWait() {
	waitErrCh := make(chan error, 1)
	c.waitCh = make(chan struct{}, 1)
	c.stopCh = make(chan struct{}, 1)

	// wait for process in go routine
	go func() {
		waitErrCh <- c.wait()
	}()

	go func() {
		if c.timeout > 0 {
			time.Sleep(c.timeout)
			if c.stopCh != nil {
				c.stopCh <- struct{}{}
			}
		}
	}()

	// watch for wait or stop
	go func() {
		defer func() {
			close(c.waitCh)
			close(waitErrCh)
		}()
		// Wait until Stop() is called or/and Wait() is returning.
		for {
			select {
			case err := <-waitErrCh:
				if c.stop {
					// Ignore error if Stop() was called.
					return
				}
				c.setWaitError(err)
				if c.WaitHandler != nil {
					c.WaitHandler(c.waitError)
				}
				return
			case <-c.stopCh:
				c.stop = true
				// Prevent next readings from the closed channel.
				c.stopCh = nil
				// The usual e.cmd.Process.Kill() is not working for the process
				// started with the new process group (Setpgid: true).
				// Negative pid number is used to send a signal to all processes in the group.
				err := c.session.Signal(ssh.SIGKILL)
				if err != nil {
					c.killError = err
				}
			}
		}
	}()
}

func (c *SSHCommand) Run(ctx context.Context) error {
	log.DebugF("executor: run '%s'\n", c.cmd)
	c.Cmd(ctx)

	if c.session == nil {
		return fmt.Errorf("ssh session not started")
	}
	defer c.session.Close()

	err := c.Start()
	if err != nil {
		return err
	}

	c.Stop()

	return c.WaitError()
}

func (c *SSHCommand) WaitError() error {
	defer c.lockWaitError.RUnlock()
	c.lockWaitError.RLock()
	return c.waitError
}

func (c *SSHCommand) StderrBytes() []byte {
	if len(c.ErrBytes.Bytes()) > 0 {
		return c.ErrBytes.Bytes()
	}

	if c.err != nil {
		return c.err.Bytes()
	}

	return nil
}

func (c *SSHCommand) StdoutBytes() []byte {
	if len(c.OutBytes.Bytes()) > 0 {
		return c.OutBytes.Bytes()
	}

	if c.out != nil {
		return c.out.Bytes()
	}

	return nil
}

func (c *SSHCommand) WithMatchers(matchers ...*process.ByteSequenceMatcher) *SSHCommand {
	c.Matchers = make([]*process.ByteSequenceMatcher, 0)
	c.Matchers = append(c.Matchers, matchers...)
	return c
}

func (c *SSHCommand) WithWaitHandler(waitHandler func(error)) *SSHCommand {
	c.WaitHandler = waitHandler
	return c
}

func (c *SSHCommand) OpenStdinPipe() *SSHCommand {
	c.StdinPipe = true
	return c
}

func (c *SSHCommand) WithMatchHandler(fn func(pattern string) string) *SSHCommand {
	c.MatchHandler = fn
	return c
}

func (c *SSHCommand) Sudo(ctx context.Context) {
	cmdLine := c.Name + " " + strings.Join(c.Args, " ")
	sudoCmdLine := fmt.Sprintf(
		`sudo -p SudoPassword -H -S -i bash -c 'echo SUDO-SUCCESS && %s'`,
		cmdLine,
	)

	c.cmd = sudoCmdLine
	c.Cmd(ctx)

	c.WithMatchers(
		process.NewByteSequenceMatcher("SudoPassword"),
		process.NewByteSequenceMatcher("SUDO-SUCCESS").WaitNonMatched(),
	)
	c.OpenStdinPipe()

	passSent := false
	c.WithMatchHandler(func(pattern string) string {
		if pattern == "SudoPassword" {
			log.DebugLn("Send become pass to cmd")
			var becomePass string

			if c.sshClient.Settings.BecomePass != "" {
				becomePass = c.sshClient.Settings.BecomePass
			} else {
				becomePass = app.BecomePass
			}
			var err error
			_, err = c.Stdin.Write([]byte(becomePass + "\n"))
			if err != nil {
				log.ErrorLn("got error from sending pass to stdin")
			}
			if !passSent {
				passSent = true
			} else {
				// Second prompt is error!
				log.ErrorLn("Bad sudo password.")
			}
			return "reset"
		}
		if pattern == "SUDO-SUCCESS" {
			log.DebugLn("Got SUCCESS")
			if c.onCommandStart != nil {
				c.onCommandStart()
			}
			return "done"
		}
		return ""
	})
}

func (c *SSHCommand) WithStdoutHandler(handler func(string)) {
	c.stdoutHandler = handler
}

func (c *SSHCommand) WithStderrHandler(handler func(string)) {
	c.stderrHandler = handler
}

func (c *SSHCommand) Cmd(ctx context.Context) {
	if ctx != nil {
		c.ctx = ctx
	}
	c.Cancel = func() error {
		return c.session.Signal(ssh.SIGINT)
	}
}

func (c *SSHCommand) Output(ctx context.Context) ([]byte, []byte, error) {
	c.Cmd(ctx)
	if c.session == nil {
		return nil, nil, fmt.Errorf("ssh session not started")
	}
	defer c.session.Close()

	if c.out == nil {
		c.out = new(bytes.Buffer)
	} else {
		c.out.Reset()
	}

	if c.err == nil {
		c.err = new(bytes.Buffer)
	} else {
		c.err.Reset()
	}

	var err error
	c.stdoutPipeFile, err = c.session.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("open stdout pipe '%s': %w", c.Name, err)
	}

	c.stderrPipeFile, err = c.session.StderrPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("open stderr pipe '%s': %w", c.Name, err)
	}

	err = c.Start()
	c.wg.Wait()
	return c.out.Bytes(), c.err.Bytes(), err
}

type singleWriter struct {
	b  bytes.Buffer
	mu sync.Mutex
}

func (w *singleWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

func (c *SSHCommand) CombinedOutput(ctx context.Context) ([]byte, error) {
	c.Cmd(ctx)
	if c.session == nil {
		return nil, fmt.Errorf("ssh session not started")
	}

	defer c.session.Close()

	if c.out == nil {
		c.out = new(bytes.Buffer)
	} else {
		c.out.Reset()
	}

	if c.err == nil {
		c.err = new(bytes.Buffer)
	} else {
		c.err.Reset()
	}

	var err error
	c.stdoutPipeFile, err = c.session.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open stdout pipe '%s': %w", c.Name, err)
	}

	c.stderrPipeFile, err = c.session.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("open stderr pipe '%s': %w", c.Name, err)
	}
	var co singleWriter
	c.combined = &co

	err = c.Start()
	c.wg.Wait()
	return c.combined.b.Bytes(), err
}

func (c *SSHCommand) WithTimeout(timeout time.Duration) {
	c.timeout = timeout
}

func (c *SSHCommand) WithEnv(env map[string]string) {
	c.Env = make([]string, 0, len(env))
	for k, v := range env {
		c.Env = append(c.Env, fmt.Sprintf("%s=%s", k, v))
	}
}
func (c *SSHCommand) CaptureStdout(buf *bytes.Buffer) *SSHCommand {
	if buf != nil {
		c.out = buf
	} else {
		c.out = &bytes.Buffer{}
	}
	return c
}

func (c *SSHCommand) CaptureStderr(buf *bytes.Buffer) *SSHCommand {
	if buf != nil {
		c.err = buf
	} else {
		c.err = &bytes.Buffer{}
	}
	return c
}

func (c *SSHCommand) SetupStreamHandlers() (err error) {
	// stderr goes to console (commented because ssh writes only "Connection closed" messages to stderr)
	// c.Cmd.Stderr = os.Stderr
	// connect console's stdin
	// c.Cmd.Stdin = os.Stdin

	// setup stdout stream handlers
	if c.session != nil && c.out == nil && c.stdoutHandler == nil && len(c.Matchers) == 0 {
		c.session.Stdout = os.Stdout
		c.session.Stdout = &c.OutBytes
		c.session.Stderr = &c.ErrBytes
		return
	}

	var stdoutHandlerWritePipe *os.File
	var stdoutHandlerReadPipe *os.File
	if c.out != nil || c.stdoutHandler != nil || len(c.Matchers) > 0 {

		if c.out == nil {
			c.out = new(bytes.Buffer)
		}

		if c.stdoutPipeFile == nil {
			var err error
			c.stdoutPipeFile, err = c.session.StdoutPipe()
			if err != nil {
				return fmt.Errorf("open stdout pipe '%s': %w", c.Name, err)
			}
		}

		// create pipe for StdoutHandler
		if c.stdoutHandler != nil {
			stdoutHandlerReadPipe, stdoutHandlerWritePipe, err = os.Pipe()
			if err != nil {
				return fmt.Errorf("unable to create os pipe for stdoutHandler: %s", err)
			}
		}
	}

	var stderrReadPipe io.Reader
	var stderrHandlerWritePipe *os.File
	var stderrHandlerReadPipe *os.File
	if c.err != nil || c.stderrHandler != nil || len(c.Matchers) > 0 {

		if c.err == nil {
			c.err = new(bytes.Buffer)
		}

		if c.stderrPipeFile == nil {
			var err error
			c.stderrPipeFile, err = c.session.StderrPipe()
			if err != nil {
				return fmt.Errorf("open stdout pipe '%s': %w", c.Name, err)
			}
		}

		// create pipe for StderrHandler
		if c.stderrHandler != nil {
			stderrHandlerReadPipe, stderrHandlerWritePipe, err = os.Pipe()
			if err != nil {
				return fmt.Errorf("unable to create os pipe for stderrHandler: %s", err)
			}
		}
	}

	if c.StdinPipe {
		c.Stdin, err = c.session.StdinPipe()
		if err != nil {
			return fmt.Errorf("open stdin pipe: %v", err)
		}
	}

	// Start reading from stdout of a command.
	// Wait until all matchers are done and then:
	// - Copy to os.Stdout if live output is enabled
	// - Copy to buffer if capture is enabled
	// - Copy to pipe if StdoutHandler is set
	c.wg.Add(2)
	go func() {
		c.readFromStreams(c.stdoutPipeFile, stdoutHandlerWritePipe, false)
	}()

	// sudo hack, becouse of password prompt is sent to STDERR, not STDOUT
	go func() {
		c.readFromStreams(c.stderrPipeFile, stdoutHandlerWritePipe, true)
	}()

	go func() {
		if c.stdoutHandler == nil {
			return
		}
		c.ConsumeLines(stdoutHandlerReadPipe, c.stdoutHandler)
		log.DebugF("stop line consumer for '%s'\n", c.cmd)
	}()

	// Start reading from stderr of a command.
	// Copy to os.Stderr if live output is enabled
	// Copy to buffer if capture is enabled
	// Copy to pipe if StderrHandler is set
	go func() {
		if stderrReadPipe == nil {
			return
		}

		log.DebugLn("Start reading from stderr pipe")
		defer log.DebugLn("Stop reading from stderr pipe")

		buf := make([]byte, 16)
		for {
			n, err := stderrReadPipe.Read(buf)

			// TODO logboek
			if app.IsDebug {
				os.Stderr.Write(buf[:n])
			}
			if c.err != nil {
				c.err.Write(buf[:n])
			}
			if c.stderrHandler != nil {
				_, _ = stderrHandlerWritePipe.Write(buf[:n])
			}

			if err == io.EOF {
				break
			}
		}
	}()

	go func() {
		if c.stderrHandler == nil {
			return
		}
		c.ConsumeLines(stderrHandlerReadPipe, c.stderrHandler)
		log.DebugF("stop sdterr line consumer for '%s'\n", c.cmd)
	}()

	return nil
}

func (c *SSHCommand) readFromStreams(stdoutReadPipe io.Reader, stdoutHandlerWritePipe io.Writer, isError bool) {
	defer log.DebugLn("stop readFromStreams")
	defer c.wg.Done()

	if stdoutReadPipe == nil || reflect.ValueOf(stdoutReadPipe).IsNil() {
		log.DebugLn("pipe is nil")
		return
	}

	log.DebugLn("Start read from streams for command: ", c.cmd)

	buf := make([]byte, 16)
	matchersDone := false
	errorsCount := 0
	for {
		n, err := stdoutReadPipe.Read(buf)
		if err != nil && err != io.EOF {
			log.DebugF("Error reading from stdout: %s\n", err)
			errorsCount++
			if errorsCount > 1000 {
				panic(fmt.Errorf("readFromStreams: too many errors, last error %v", err))
			}
			continue
		}

		m := 0
		if !matchersDone {
			for _, matcher := range c.Matchers {
				m = matcher.Analyze(buf[:n])
				if matcher.IsMatched() {
					log.DebugF("Trigger matcher '%s'\n", matcher.Pattern)
					// matcher is triggered
					if c.MatchHandler != nil {
						res := c.MatchHandler(matcher.Pattern)
						if res == "done" {
							matchersDone = true
							break
						}
						if res == "reset" {
							matcher.Reset()
						}
					}
				}
			}

			// stdout for internal use, no copying to pipes until all Matchers are matched
			if !matchersDone {
				m = n
			}
		}
		// TODO logboek
		if app.IsDebug {
			os.Stdout.Write(buf[m:n])
		}
		if c.out != nil && !isError {
			c.out.Write(buf[:n])
		}

		if c.err != nil && isError {
			c.err.Write(buf[:n])
		}

		if c.combined != nil {
			c.combined.Write(buf[:n])
		}
		if c.stdoutHandler != nil {
			_, _ = stdoutHandlerWritePipe.Write(buf[m:n])
		}

		if err == io.EOF {
			log.DebugLn("readFromStreams: EOF")
			break
		}
	}
}

func (c *SSHCommand) ConsumeLines(r io.Reader, fn func(l string)) {
	scanner := bufio.NewScanner(r)
	if c.StdoutSplitter != nil {
		scanner.Split(c.StdoutSplitter)
	}
	for scanner.Scan() {
		text := scanner.Text()

		if fn != nil {
			fn(text)
		}

		if text != "" {
			log.DebugF("%s: %s\n", c.cmd, text)
		}
	}
}

func (c *SSHCommand) Stop() {
	if c.stop {
		log.DebugF("Stop '%s': already stopped\n", c.cmd)
		return
	}
	if c.session == nil {
		log.DebugF("Stop '%s': session not started yet\n", c.cmd)
		return
	}
	if c.cmd == "" {
		log.DebugF("Possible BUG: Call Executor.Stop with Cmd==nil\n")
		return
	}

	c.stop = true
	log.DebugF("Stop '%s'\n", c.cmd)
	if c.stopCh != nil {
		close(c.stopCh)
	}
	log.DebugF("Stopped '%s' \n", c.cmd)
	log.DebugF("Sending SIGINT to process '%s'\n", c.cmd)
	c.session.Signal(ssh.SIGINT)
	log.DebugF("Signal sent\n")
	c.session.Signal(ssh.SIGKILL)
}

func (c *SSHCommand) setWaitError(err error) {
	defer c.lockWaitError.Unlock()
	c.lockWaitError.Lock()
	c.waitError = err
}
