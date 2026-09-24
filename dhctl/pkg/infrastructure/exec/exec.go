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

package exec

import (
	"bufio"
	"bytes"
	"container/ring"
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

const (
	HasChangesExitCode = 2

	// ForcedStopTimeout is the longest a canceled utility runs with forced stop enabled:
	// abandoned at the very end of the grace period, it gets the signal gap on top.
	ForcedStopTimeout = forcedStopGracePeriod + forcedStopSignalGap + forcedStopKillDelay

	// forcedStopGracePeriod is how long a canceled utility may keep finishing the
	// resource operations in flight before it is told to exit immediately.
	forcedStopGracePeriod = 10 * time.Minute
	// forcedStopKillDelay is how long a utility may take to exit after that.
	forcedStopKillDelay = 30 * time.Second
	// forcedStopSignalGap separates the second SIGINT from the first one when the
	// grace period is skipped: signals do not queue, and one sent right after the
	// first is merged into it.
	forcedStopSignalGap = time.Second
)

// tailLines is the number of recent stdout/stderr lines we hold onto so we can
// surface them when the subprocess exits non-zero and the unfiltered error
// buffer turned out empty (e.g. tofu / provider printed the actual failure as
// `[INFO]` or `[ERROR]` structured logs that infrastructureLogsMatcher diverts
// to the debug log file only).
const tailLines = 50

// based on https://stackoverflow.com/a/43931246
// https://regex101.com/r/qtIrSj/1
var infrastructureLogsMatcher = regexp.MustCompile(`(\s+\[(TRACE|DEBUG|INFO|WARN|ERROR)\]\s+|Use TF_LOG=TRACE|there is no package|\-\-\-\-)`)

// ringTail accumulates the last N lines fed to it. Used as a last-resort
// context source when a subprocess crashes and we have nothing more specific.
type ringTail struct {
	r *ring.Ring
}

func newRingTail(n int) *ringTail { return &ringTail{r: ring.New(n)} }

func (t *ringTail) Add(line string) {
	t.r.Value = line
	t.r = t.r.Next()
}

func (t *ringTail) String() string {
	var b strings.Builder
	t.r.Do(func(v any) {
		if s, ok := v.(string); ok && s != "" {
			b.WriteString(s)
			b.WriteByte('\n')
		}
	})
	return b.String()
}

// WithForcedStop bounds the stop of an infrastructure utility canceled through ctx.
//
// Canceling the context sends SIGINT (see cmd.Cancel in tofu and terraform), and the
// utility then finishes the resource operations in flight, which may take forever.
// With forced stop, a second SIGINT after a grace period makes it exit immediately,
// and SIGKILL to its process group, provider plugins included, handles one that still
// does not exit. The price is a possibly incomplete state: resources being changed at
// that moment may be missing from it.
//
// Closing abandoned skips the grace period once ctx is canceled: nobody waits for the
// utility to finish its work anymore, and it only delays a retry of the operation and
// may overlap it. A nil abandoned never skips the grace period.
//
// Intended for server mode, where nobody is there to press Ctrl-C twice.
func WithForcedStop(ctx context.Context, abandoned <-chan struct{}) context.Context {
	return withForcedStop(ctx, forcedStop{
		gracePeriod: forcedStopGracePeriod,
		signalGap:   forcedStopSignalGap,
		killDelay:   forcedStopKillDelay,
		abandoned:   abandoned,
	})
}

func Exec(ctx context.Context, cmd *exec.Cmd, isDebug bool) (int, error) {
	// Start infrastructure utility as a leader of the new process group to prevent
	// os.Interrupt (SIGINT) signal from the shell when Ctrl-C is pressed.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	var (
		stdout io.ReadCloser
		stderr io.ReadCloser
		err    error
	)
	if cmd.Stdout == nil {
		stdout, err = cmd.StdoutPipe()
		if err != nil {
			return 1, fmt.Errorf("stdout pipe: %v", err)
		}
		defer stdout.Close()
	}

	if cmd.Stderr == nil {
		stderr, err = cmd.StderrPipe()
		if err != nil {
			return 1, fmt.Errorf("stderr pipe: %v", err)
		}
		defer stderr.Close()
	}

	dhlog.FromContext(ctx).DebugContext(ctx, cmd.String())
	var (
		wg        sync.WaitGroup
		errBuf    bytes.Buffer
		stderrMu  sync.Mutex
		stderrAll = newRingTail(tailLines)
		stdoutMu  sync.Mutex
		stdoutAll = newRingTail(tailLines)
	)

	wg.Add(2)

	go func() {
		defer wg.Done()
		if stderr == nil {
			return
		}

		e := bufio.NewScanner(stderr)
		// terraform / tofu can emit huge JSON-ish log lines (1+ MB) when TF_LOG is on; the
		// default 64 KB scan buffer truncates them and bufio.Scanner returns an error,
		// which currently silently kills this goroutine and leaves errBuf empty.
		e.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for e.Scan() {
			txt := e.Text()
			dhlog.FromContext(ctx).DebugContext(ctx, txt)

			stderrMu.Lock()
			stderrAll.Add(txt)
			stderrMu.Unlock()

			if !isDebug {
				if !infrastructureLogsMatcher.MatchString(txt) {
					errBuf.WriteString(txt + "\n")
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		if stdout == nil {
			return
		}

		s := bufio.NewScanner(stdout)
		s.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for s.Scan() {
			line := s.Text()
			dhlog.FromContext(ctx).InfoContext(ctx, line)
			stdoutMu.Lock()
			stdoutAll.Add(line)
			stdoutMu.Unlock()
		}
	}()

	err = cmd.Start()
	if err != nil {
		dhlog.FromContext(ctx).ErrorContext(ctx, fmt.Sprintf("Cannot start cmd: %v", err))
		return cmd.ProcessState.ExitCode(), err
	}

	finishForcedStop := watchForcedStop(ctx, cmd.Process.Pid)

	wg.Wait()

	err = cmd.Wait()
	forced := finishForcedStop()

	exitCode := cmd.ProcessState.ExitCode() // 2 = exit code, if infrastructure plan has diff
	if err != nil && exitCode != HasChangesExitCode {
		dhlog.FromContext(ctx).ErrorContext(ctx, fmt.Sprintf("Error while process exit code: %v", err))
		if isDebug {
			err = fmt.Errorf("infrastructure utility has failed in DEBUG mode, search the output above for an error")
		} else {
			err = buildExitError(exitCode, &errBuf, stderrAll, stdoutAll)
		}
	}

	if exitCode == 0 {
		err = nil
	}

	if err != nil && forced {
		err = fmt.Errorf("infrastructure utility was force stopped after cancellation, "+
			"its state may miss the resources that were being changed: %w", err)
	}

	return exitCode, err
}

// buildExitError constructs an error message that always carries SOME context
// for the operator. Priority:
//  1. The unfiltered stderr (lines that didn't match `infrastructureLogsMatcher`)
//     — this is where tofu writes user-facing errors like "Error: ...".
//  2. If empty, the tail of stderr (including filtered structured logs) — covers
//     the case where a provider plugin crashed and printed only `[ERROR]` lines.
//  3. If stderr was empty too (e.g. process died from a signal before logging),
//     the tail of stdout — last visible progress before the crash.
//
// Always appends the exit code so failures coming through tofuCmd.Cancel
// (SIGINT delivered by ctx cancel) are visibly tagged.
func buildExitError(exitCode int, errBuf *bytes.Buffer, stderrAll, stdoutAll *ringTail) error {
	body := strings.TrimSpace(errBuf.String())
	if body == "" {
		body = strings.TrimSpace(stderrAll.String())
	}
	if body == "" {
		body = strings.TrimSpace(stdoutAll.String())
	}
	if body == "" {
		body = "(no stderr/stdout captured before crash; rerun with --debug or check the debug log file)"
	}
	return fmt.Errorf("infrastructure utility exited with code %d:\n%s", exitCode, body)
}

func ReplaceHomeDirEnv(env []string, homeDir string) []string {
	res := make([]string, 0, len(env))
	for _, e := range env {
		v := e
		if strings.HasPrefix(e, "HOME=") {
			v = fmt.Sprintf("HOME=%s", homeDir)
		}
		res = append(res, v)
	}

	return res
}

type forcedStopKey struct{}

type forcedStop struct {
	gracePeriod time.Duration
	signalGap   time.Duration
	killDelay   time.Duration
	abandoned   <-chan struct{}
}

func withForcedStop(ctx context.Context, params forcedStop) context.Context {
	return context.WithValue(ctx, forcedStopKey{}, params)
}

// watchForcedStop escalates the stop of the process group led by pid once ctx is
// canceled, if forced stop is enabled. The returned func must be called after the
// process is waited for: it stops the watcher, waits for it, and reports whether
// the stop was forced.
func watchForcedStop(ctx context.Context, pid int) func() bool {
	params, ok := ctx.Value(forcedStopKey{}).(forcedStop)
	if !ok {
		return func() bool { return false }
	}

	exited := make(chan struct{})
	forced := false

	var wg sync.WaitGroup
	wg.Go(func() {
		select {
		case <-exited:
			return
		case <-ctx.Done():
		}

		steps := []struct {
			after  time.Duration
			skip   <-chan struct{} // a nil channel never skips the wait
			signal syscall.Signal
			name   string
		}{
			{after: params.gracePeriod, skip: params.abandoned, signal: syscall.SIGINT, name: "SIGINT"},
			{after: params.killDelay, signal: syscall.SIGKILL, name: "SIGKILL"},
		}

		for _, step := range steps {
			reason := fmt.Sprintf("did not exit %s after the previous signal", step.after)
			select {
			case <-exited:
				return
			case <-time.After(step.after):
			case <-step.skip:
				reason = "is abandoned"

				// When abandoned is done together with ctx, cmd.Cancel sends the first
				// SIGINT right now: keep the second one apart from it.
				select {
				case <-exited:
					return
				case <-time.After(params.signalGap):
				}
			}

			// The utility may have exited already and be waiting to be reaped: a zombie
			// takes the signal without an error, so the stop is reported as forced
			// although it was not. Such a stop happens only right at the step.
			forced = true
			dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf(
				"Infrastructure utility (pid %d) %s, sending %s", pid, reason, step.name,
			))
			_ = syscall.Kill(-pid, step.signal)
		}
	})

	return func() bool {
		close(exited)
		wg.Wait()

		return forced
	}
}
