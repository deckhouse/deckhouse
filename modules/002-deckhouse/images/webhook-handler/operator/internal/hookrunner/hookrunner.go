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

// Package hookrunner runs Python hooks on a long-lived interpreter.
//
// Starting python3 for every admission or conversion request costs 24-32 ms, of which the hook
// itself is about 2 ms; the rest is interpreter start-up and imports. python_hook_server.py pays
// that once and forks per request instead. Runner hands it the Python hooks and starts every
// other hook as a process, as shell-operator does by default.
package hookrunner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/flant/shell-operator/pkg/executor"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// restartDelay is the pause before the server is started again after it exits.
const restartDelay = time.Second

type Runner struct {
	python     string
	serverPath string
	socketPath string
	logger     *log.Logger

	process executor.ProcessRunner
}

// New returns a Runner whose server is `python serverPath socketPath`; Serve starts it.
func New(python, serverPath, socketPath string, logger *log.Logger) *Runner {
	return &Runner{
		python:     python,
		serverPath: serverPath,
		socketPath: socketPath,
		logger:     logger,
	}
}

// Serve runs the server until ctx is done and starts it again whenever it exits. While it is
// down, Run starts Python hooks as processes.
func (r *Runner) Serve(ctx context.Context) {
	for {
		cmd := exec.CommandContext(ctx, r.python, r.serverPath, r.socketPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()
		if ctx.Err() != nil {
			return
		}
		r.logger.Error("python hook server exited, starting it again", log.Err(err))

		select {
		case <-ctx.Done():
			return
		case <-time.After(restartDelay):
		}
	}
}

// Run implements executor.Runner.
func (r *Runner) Run(ctx context.Context, cmd *exec.Cmd) (*executor.CmdUsage, error) {
	if !isPythonScript(cmd.Path) {
		return r.process.Run(ctx, cmd)
	}

	conn, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: r.socketPath, Net: "unix"})
	if err != nil {
		r.logger.Warn("python hook server is unavailable, starting the hook as a process",
			slog.String("hook", cmd.Path), log.Err(err))
		return r.process.Run(ctx, cmd)
	}
	defer conn.Close()

	return r.runOnServer(ctx, conn, cmd)
}

type request struct {
	Path string            `json:"path"`
	Args []string          `json:"args"`
	Env  map[string]string `json:"env"`
	Dir  string            `json:"dir"`
}

type result struct {
	ExitCode int     `json:"exit_code"`
	UTime    float64 `json:"utime"`
	STime    float64 `json:"stime"`
	MaxRSS   int64   `json:"maxrss"`
}

func (r *Runner) runOnServer(ctx context.Context, conn *net.UnixConn, cmd *exec.Cmd) (*executor.CmdUsage, error) {
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	defer stdoutR.Close()

	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		stdoutW.Close()
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}
	defer stderrR.Close()

	sendErr := sendRequest(conn, cmd, stdoutW, stderrW)
	// The hook holds its own copies now; ours must go, or the pipes never reach EOF.
	stdoutW.Close()
	stderrW.Close()
	if sendErr != nil {
		// Nothing reached the server, so the hook has not run: start it the usual way.
		r.logger.Warn("python hook server did not take the request, starting the hook as a process",
			slog.String("hook", cmd.Path), log.Err(sendErr))
		return r.process.Run(ctx, cmd)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go copyOutput(&wg, cmd.Stdout, stdoutR)
	go copyOutput(&wg, cmd.Stderr, stderrR)

	line, readErr := bufio.NewReader(conn).ReadBytes('\n')
	wg.Wait()

	if readErr != nil {
		return nil, fmt.Errorf("cmd run: hook ended without a result, it was likely killed: %w", readErr)
	}

	var res result
	if err := json.Unmarshal(line, &res); err != nil {
		return nil, fmt.Errorf("cmd run: hook result: %w", err)
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("cmd run: exit status %d", res.ExitCode)
	}

	return &executor.CmdUsage{
		User:   seconds(res.UTime),
		Sys:    seconds(res.STime),
		MaxRss: res.MaxRSS,
	}, nil
}

func sendRequest(conn *net.UnixConn, cmd *exec.Cmd, stdout, stderr *os.File) error {
	args := []string{}
	if len(cmd.Args) > 1 {
		args = cmd.Args[1:]
	}

	payload, err := json.Marshal(request{Path: cmd.Path, Args: args, Env: envMap(cmd.Env), Dir: cmd.Dir})
	if err != nil {
		return err
	}

	msg := binary.BigEndian.AppendUint32(nil, uint32(len(payload)))
	msg = append(msg, payload...)
	rights := syscall.UnixRights(int(stdout.Fd()), int(stderr.Fd()))

	n, _, err := conn.WriteMsgUnix(msg, rights, nil)
	if err != nil {
		return err
	}
	if n < len(msg) {
		_, err = conn.Write(msg[n:])
	}

	return err
}

// envMap keeps the last value of a repeated variable, as exec.Cmd does.
func envMap(env []string) map[string]string {
	m := make(map[string]string, len(env))
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		m[k] = v
	}

	return m
}

func copyOutput(wg *sync.WaitGroup, dst io.Writer, src io.Reader) {
	defer wg.Done()
	if dst == nil {
		dst = io.Discard
	}
	_, _ = io.Copy(dst, src)
}

func seconds(s float64) time.Duration {
	return time.Duration(s * float64(time.Second))
}

// isPythonScript reports whether the file starts with a python shebang, which is what decides
// the interpreter when shell-operator executes it.
func isPythonScript(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	head := make([]byte, 128)
	n, err := f.Read(head)
	if err != nil && !errors.Is(err, io.EOF) {
		return false
	}
	firstLine, _, _ := bytes.Cut(head[:n], []byte("\n"))

	return bytes.HasPrefix(firstLine, []byte("#!")) && bytes.Contains(firstLine, []byte("python"))
}
