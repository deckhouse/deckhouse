package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/ssh/util"
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
	Cmd *exec.Cmd

	Live bool

	Sudo      bool
	StdinPipe bool

	Stdin io.WriteCloser

	Matchers     []*util.ByteSequenceMatcher
	MatchHandler func(pattern string) string

	StdoutBuffer   *bytes.Buffer
	StdoutSplitter bufio.SplitFunc
	StdoutHandler  func(l string)

	StderrBuffer *bytes.Buffer

	stop bool
}

func NewExecutor(cmd *exec.Cmd) *Executor {
	return &Executor{
		Cmd: cmd,
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

func (e *Executor) CaptureStdout(buf *bytes.Buffer) *Executor {
	if buf != nil {
		e.StdoutBuffer = buf
	} else {
		e.StdoutBuffer = &bytes.Buffer{}
	}
	return e
}

func (e *Executor) WithMatchers(matchers ...*util.ByteSequenceMatcher) *Executor {
	e.Matchers = make([]*util.ByteSequenceMatcher, 0)
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

func (e *Executor) SetupStreamHandlers() (err error) {
	// stderr goes to console (commented because ssh writes only "Connection closed" messages to stderr)
	// e.Cmd.Stderr = os.Stderr
	// connect console's stdin
	//e.Cmd.Stdin = os.Stdin

	// setup stdout stream handlers
	if e.Live && e.StdoutBuffer == nil && e.StdoutHandler == nil && len(e.Matchers) == 0 {
		e.Cmd.Stdout = os.Stdout
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
		e.Cmd.Stdout = stdoutWritePipe

		// create pipe for StdoutHandler
		if e.StdoutHandler != nil {
			stdoutHandlerReadPipe, stdoutHandlerWritePipe, err = os.Pipe()
			if err != nil {
				return fmt.Errorf("unable to create os pipe for stdoutHandler: %s", err)
			}
		}
	}

	if e.StdinPipe {
		e.Stdin, err = e.Cmd.StdinPipe()
		if err != nil {
			return fmt.Errorf("open stdin pipe: %v", err)
		}
	}

	// Start reading from stdout of a command.
	// Copy to os.Stdout if live output is enabled
	// Wait for SUCCESS line if sudo is enabled and then:
	// Copy to buffer if capture is enabled
	// Copy to pipe if StdoutHandler is set
	//var cmdStdoutOutput bytes.Buffer

	go func() {
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
						app.Debugf("Trigger matcher '%s'\n", matcher.Pattern)
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
			if app.IsDebug == 1 {
				os.Stdout.Write(buf[:n])
			}
			if e.Live {
				os.Stdout.Write(buf[m:n])
			}
			if e.StdoutBuffer != nil {
				e.StdoutBuffer.Write(buf[m:n])
			}
			if e.StdoutHandler != nil {
				stdoutHandlerWritePipe.Write(buf[m:n])
			}

			if err == io.EOF {
				break
			}
		}
	}()

	if e.StdoutHandler != nil {
		go func() {
			e.ConsumeLines(stdoutHandlerReadPipe, e.StdoutHandler)
			app.Debugf("stop line consumer for '%s'\n", e.Cmd.Args[0])
		}()
	}

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

		if app.IsDebug == 1 && text != "" {
			fmt.Printf("%s: %s\n", e.Cmd.Args[0], text)
		}
	}
}

func (e *Executor) Start() error {
	// setup stream handlers
	app.Debugf("start: %s", e.Cmd.String())
	err := e.SetupStreamHandlers()
	if err != nil {
		return err
	}
	return e.Cmd.Start()
}

func (e *Executor) Stop() error {
	e.stop = true
	e.Cmd.Process.Kill()
	return e.Cmd.Wait()
}

func (e *Executor) Run() error {
	app.Debugf("run: %s", e.Cmd.String())
	// setup stream handlers
	err := e.SetupStreamHandlers()
	if err != nil {
		return err
	}
	return e.Cmd.Run()
}
