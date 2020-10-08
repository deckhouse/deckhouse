package process

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"

	"flant/candictl/pkg/app"
	"flant/candictl/pkg/log"
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

	StderrBuffer   *bytes.Buffer
	StderrSplitter bufio.SplitFunc
	StderrHandler  func(l string)

	WaitHandler func(err error)

	started   bool
	stop      bool
	waitCh    chan struct{}
	stopCh    chan struct{}
	waitError error
	killError error
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

func (e *Executor) WithStdoutHandler(stdoutHandler func(l string)) *Executor {
	e.StdoutHandler = stdoutHandler
	return e
}

func (e *Executor) WithStdoutSplitter(fn bufio.SplitFunc) *Executor {
	e.StdoutSplitter = fn
	return e
}

func (e *Executor) WithStderrHandler(stderrHandler func(l string)) *Executor {
	e.StderrHandler = stderrHandler
	return e
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
		if stdoutReadPipe == nil {
			return
		}
		buf := make([]byte, 16)
		matchersDone := false
		if len(e.Matchers) == 0 {
			matchersDone = true
		}
		for {
			n, err := stdoutReadPipe.Read(buf)

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
				break
			}
		}
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
				e.waitError = err
				if e.WaitHandler != nil {
					e.WaitHandler(e.waitError)
				}
				// close(e.waitCh)
				return
			case <-e.stopCh:
				e.stop = true
				// prevent next readings from closed channel
				e.stopCh = nil
				err := e.cmd.Process.Kill()
				if err != nil {
					e.killError = err
				}
			}
		}
	}()
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

	log.DebugF("Stop '%s'\n", e.cmd.String())
	if e.stopCh != nil {
		close(e.stopCh)
	}
	<-e.waitCh

	log.DebugF("Stopped '%s': %d\n", e.cmd.String(), e.cmd.ProcessState.ExitCode())
}

// Run executes a command and blocks until it is finished or stopped.
func (e *Executor) Run() error {
	log.DebugF("executor: run '%s'\n", e.cmd.String())

	err := e.Start()
	if err != nil {
		return err
	}
	<-e.waitCh
	return e.waitError
}

func (e *Executor) Cmd() *exec.Cmd {
	return e.cmd
}
