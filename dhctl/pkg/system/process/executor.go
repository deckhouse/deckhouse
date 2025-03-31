// Copyright 2021 Flant JSC
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

package process

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"sync"
	"syscall"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

// exec.Cmd executor

/*
4 types for stdout:

live output or not.  copy to os.Stdout or not

wait for SUCCESS line?

capture output.  copy to bytes.Buffer or not

stdout handler.  buffered pipe to read output line by line with scanner.Scan


2 types for stderr

live stderr. Copy to os.Stderr or be quiet.

capture stderr. copy to bytes.Buffer For errors!


2 types of running:

execute and wait while finished

start in a background



What combinations do we really need?

SudoStart:    (kubectl proxy)
- live stderr
- live stdout until some line occurs
- wait for SUCCESS line
- stdout handler

SudoRun:     (bashible bundle, etc)
- live stdout and stderr
- capture stdout


Start:     (tunnels, agent)
- no live
- stdout handler
- wait until success


LiveRun:   (ssh-add)
- live output
- no capture

LiveOutput

NewCommand("ls", "-la").Live().Sudo().


proxyCmd := NewCommand("kube-proxy").EnableSudo().EnableLive().CaptureStdout(buf)

// run in background
err := proxyCmd.Start()

*/

type Executor struct {
	cmd *exec.Cmd

	Session *Session

	Live      bool
	StdinPipe bool

	Stdin io.WriteCloser

	Matchers     []*ByteSequenceMatcher
	MatchHandler func(pattern string) string

	StdoutBuffer   *bytes.Buffer
	StdoutSplitter bufio.SplitFunc
	StdoutHandler  func(l string)

	pipesMutex     sync.Mutex
	stdoutPipeFile *os.File
	stderrPipeFile *os.File

	StderrBuffer   *bytes.Buffer
	StderrSplitter bufio.SplitFunc
	StderrHandler  func(l string)

	WaitHandler func(err error)

	started bool
	stop    bool
	waitCh  chan struct{}
	stopCh  chan struct{}

	lockWaitError sync.RWMutex
	waitError     error

	killError error

	timeout time.Duration
}

func NewDefaultExecutor(cmd *exec.Cmd) *Executor {
	return NewExecutor(DefaultSession, cmd)
}

func NewExecutor(sess *Session, cmd *exec.Cmd) *Executor {
	return &Executor{
		Session: sess,
		cmd:     cmd,
	}
}

func (e *Executor) EnableLive() *Executor {
	e.Live = true
	return e
}

func (e *Executor) OpenStdinPipe() *Executor {
	e.StdinPipe = true
	return e
}

func (e *Executor) WithStdoutHandler(stdoutHandler func(l string)) {
	e.StdoutHandler = stdoutHandler
}

func (e *Executor) WithStdoutSplitter(fn bufio.SplitFunc) *Executor {
	e.StdoutSplitter = fn
	return e
}

func (e *Executor) WithStderrHandler(stderrHandler func(l string)) {
	e.StderrHandler = stderrHandler
}

func (e *Executor) WithStderrSplitter(fn bufio.SplitFunc) *Executor {
	e.StderrSplitter = fn
	return e
}

func (e *Executor) WithWaitHandler(waitHandler func(error)) *Executor {
	e.WaitHandler = waitHandler
	return e
}

func (e *Executor) CaptureStdout(buf *bytes.Buffer) *Executor {
	if buf != nil {
		e.StdoutBuffer = buf
	} else {
		e.StdoutBuffer = &bytes.Buffer{}
	}
	return e
}

func (e *Executor) CaptureStderr(buf *bytes.Buffer) *Executor {
	if buf != nil {
		e.StderrBuffer = buf
	} else {
		e.StderrBuffer = &bytes.Buffer{}
	}
	return e
}

func (e *Executor) WithTimeout(timeout time.Duration) *Executor {
	e.timeout = timeout
	return e
}

func (e *Executor) WithMatchers(matchers ...*ByteSequenceMatcher) *Executor {
	e.Matchers = make([]*ByteSequenceMatcher, 0)
	e.Matchers = append(e.Matchers, matchers...)
	return e
}

func (e *Executor) WithMatchHandler(fn func(pattern string) string) *Executor {
	e.MatchHandler = fn
	return e
}

func (e *Executor) StdoutBytes() []byte {
	if e.StdoutBuffer != nil {
		return e.StdoutBuffer.Bytes()
	}
	return nil
}

func (e *Executor) StderrBytes() []byte {
	if e.StderrBuffer != nil {
		return e.StderrBuffer.Bytes()
	}
	return nil
}

func (e *Executor) SetupStreamHandlers() (err error) {
	// stderr goes to console (commented because ssh writes only "Connection closed" messages to stderr)
	// e.Cmd.Stderr = os.Stderr
	// connect console's stdin
	// e.Cmd.Stdin = os.Stdin

	// setup stdout stream handlers
	if e.Live && e.StdoutBuffer == nil && e.StdoutHandler == nil && len(e.Matchers) == 0 {
		e.cmd.Stdout = os.Stdout
		return
	}

	var stdoutReadPipe *os.File
	var stdoutHandlerWritePipe *os.File
	var stdoutHandlerReadPipe *os.File
	if e.StdoutBuffer != nil || e.StdoutHandler != nil || len(e.Matchers) > 0 {
		// create pipe for stdout
		var stdoutWritePipe *os.File
		stdoutReadPipe, stdoutWritePipe, err = os.Pipe()
		if err != nil {
			return fmt.Errorf("unable to create os pipe for stdout: %s", err)
		}

		e.cmd.Stdout = stdoutWritePipe

		e.pipesMutex.Lock()
		e.stdoutPipeFile = stdoutWritePipe
		e.pipesMutex.Unlock()

		// create pipe for StdoutHandler
		if e.StdoutHandler != nil {
			stdoutHandlerReadPipe, stdoutHandlerWritePipe, err = os.Pipe()
			if err != nil {
				return fmt.Errorf("unable to create os pipe for stdoutHandler: %s", err)
			}
		}
	}

	var stderrReadPipe *os.File
	var stderrHandlerWritePipe *os.File
	var stderrHandlerReadPipe *os.File
	if e.StderrBuffer != nil || e.StderrHandler != nil {
		// create pipe for stderr
		var stderrWritePipe *os.File
		stderrReadPipe, stderrWritePipe, err = os.Pipe()
		if err != nil {
			return fmt.Errorf("unable to create os pipe for stderr: %s", err)
		}
		e.cmd.Stderr = stderrWritePipe

		e.pipesMutex.Lock()
		e.stderrPipeFile = stderrWritePipe
		e.pipesMutex.Unlock()

		// create pipe for StderrHandler
		if e.StderrHandler != nil {
			stderrHandlerReadPipe, stderrHandlerWritePipe, err = os.Pipe()
			if err != nil {
				return fmt.Errorf("unable to create os pipe for stderrHandler: %s", err)
			}
		}
	}

	if e.StdinPipe {
		e.Stdin, err = e.cmd.StdinPipe()
		if err != nil {
			return fmt.Errorf("open stdin pipe: %v", err)
		}
	}

	// Start reading from stdout of a command.
	// Wait until all matchers are done and then:
	// - Copy to os.Stdout if live output is enabled
	// - Copy to buffer if capture is enabled
	// - Copy to pipe if StdoutHandler is set
	go func() {
		e.readFromStreams(stdoutReadPipe, stdoutHandlerWritePipe)
	}()

	go func() {
		if e.StdoutHandler == nil {
			return
		}
		e.ConsumeLines(stdoutHandlerReadPipe, e.StdoutHandler)
		log.DebugF("stop line consumer for '%s'\n", e.cmd.Args[0])
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
			if e.Live || app.IsDebug {
				os.Stderr.Write(buf[:n])
			}
			if e.StderrBuffer != nil {
				e.StderrBuffer.Write(buf[:n])
			}
			if e.StderrHandler != nil {
				_, _ = stderrHandlerWritePipe.Write(buf[:n])
			}

			if err == io.EOF {
				break
			}
		}
	}()

	go func() {
		if e.StderrHandler == nil {
			return
		}
		e.ConsumeLines(stderrHandlerReadPipe, e.StderrHandler)
		log.DebugF("stop sdterr line consumer for '%s'\n", e.cmd.Args[0])
	}()

	return nil
}

func (e *Executor) readFromStreams(stdoutReadPipe io.Reader, stdoutHandlerWritePipe io.Writer) {
	defer log.DebugLn("stop readFromStreams")

	if stdoutReadPipe == nil || reflect.ValueOf(stdoutReadPipe).IsNil() {
		return
	}

	log.DebugLn("Start read from streams for command: ", e.cmd.String())

	buf := make([]byte, 16)
	matchersDone := false
	if len(e.Matchers) == 0 {
		matchersDone = true
	}

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
			for _, matcher := range e.Matchers {
				m = matcher.Analyze(buf[:n])
				if matcher.IsMatched() {
					log.DebugF("Trigger matcher '%s'\n", matcher.Pattern)
					// matcher is triggered
					if e.MatchHandler != nil {
						res := e.MatchHandler(matcher.Pattern)
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
			os.Stdout.Write(buf[:n])
		}
		if e.Live {
			os.Stdout.Write(buf[m:n])
		}
		if e.StdoutBuffer != nil {
			e.StdoutBuffer.Write(buf[m:n])
		}
		if e.StdoutHandler != nil {
			_, _ = stdoutHandlerWritePipe.Write(buf[m:n])
		}

		if err == io.EOF {
			log.DebugLn("readFromStreams: EOF")
			break
		}
	}
}

func (e *Executor) ConsumeLines(r io.Reader, fn func(l string)) {
	scanner := bufio.NewScanner(r)
	if e.StdoutSplitter != nil {
		scanner.Split(e.StdoutSplitter)
	}
	for scanner.Scan() {
		text := scanner.Text()

		if fn != nil {
			fn(text)
		}

		if text != "" {
			log.DebugF("%s: %s\n", e.cmd.Args[0], text)
		}
	}
}

func (e *Executor) Start() error {
	// setup stream handlers
	log.DebugF("executor: start '%s'\n", e.cmd.String())
	err := e.SetupStreamHandlers()
	if err != nil {
		return err
	}

	err = e.cmd.Start()
	if err != nil {
		return err
	}
	e.started = true

	e.ProcessWait()

	log.DebugF("Register stoppable: '%s'\n", e.cmd.String())
	e.Session.RegisterStoppable(e)

	return nil
}

func (e *Executor) ProcessWait() {
	waitErrCh := make(chan error, 1)
	e.waitCh = make(chan struct{}, 1)
	e.stopCh = make(chan struct{}, 1)

	// wait for process in go routine
	go func() {
		waitErrCh <- e.cmd.Wait()
	}()

	go func() {
		if e.timeout > 0 {
			time.Sleep(e.timeout)
			if e.stopCh != nil {
				e.stopCh <- struct{}{}
			}
		}
	}()

	// watch for wait or stop
	go func() {
		defer func() {
			close(e.waitCh)
			close(waitErrCh)
		}()
		// Wait until Stop() is called or/and Wait() is returning.
		for {
			select {
			case err := <-waitErrCh:
				if e.stop {
					// Ignore error if Stop() was called.
					// close(e.waitCh)
					return
				}
				e.setWaitError(err)
				if e.WaitHandler != nil {
					e.WaitHandler(e.waitError)
				}
				// close(e.waitCh)
				return
			case <-e.stopCh:
				e.stop = true
				// Prevent next readings from the closed channel.
				e.stopCh = nil
				// The usual e.cmd.Process.Kill() is not working for the process
				// started with the new process group (Setpgid: true).
				// Negative pid number is used to send a signal to all processes in the group.
				err := syscall.Kill(-e.cmd.Process.Pid, syscall.SIGKILL)
				if err != nil {
					e.killError = err
				}
			}
		}
	}()
}

func (e *Executor) closePipes() {
	log.DebugLn("Starting close piped")
	defer log.DebugLn("Stop close piped")

	e.pipesMutex.Lock()
	defer e.pipesMutex.Unlock()

	if e.stdoutPipeFile != nil {
		err := e.stdoutPipeFile.Close()
		if err != nil {
			log.DebugF("Cannot close stdout pipe: %v\n", err)
		}
		e.stdoutPipeFile = nil
	}

	if e.stderrPipeFile != nil {
		err := e.stderrPipeFile.Close()
		if err != nil {
			log.DebugF("Cannot close stderr pipe: %v\n", err)
		}
		e.stderrPipeFile = nil
	}
}

func (e *Executor) Stop() {
	if e.stop {
		log.DebugF("Stop '%s': already stopped\n", e.cmd.String())
		return
	}
	if !e.started {
		log.DebugF("Stop '%s': not started yet\n", e.cmd.String())
		return
	}
	if e.cmd == nil {
		log.DebugF("Possible BUG: Call Executor.Stop with Cmd==nil\n")
		return
	}

	e.stop = true
	log.DebugF("Stop '%s'\n", e.cmd.String())
	if e.stopCh != nil {
		close(e.stopCh)
	}
	<-e.waitCh
	log.DebugF("Stopped '%s': %d\n", e.cmd.String(), e.cmd.ProcessState.ExitCode())
	e.closePipes()
}

// Run executes a command and blocks until it is finished or stopped.
func (e *Executor) Run(_ context.Context) error {
	log.DebugF("executor: run '%s'\n", e.cmd.String())

	err := e.Start()
	if err != nil {
		return err
	}

	<-e.waitCh

	e.closePipes()

	return e.WaitError()
}

func (e *Executor) Cmd() *exec.Cmd {
	return e.cmd
}

func (e *Executor) setWaitError(err error) {
	defer e.lockWaitError.Unlock()
	e.lockWaitError.Lock()
	e.waitError = err
}

func (e *Executor) WaitError() error {
	defer e.lockWaitError.RUnlock()
	e.lockWaitError.RLock()
	return e.waitError
}
