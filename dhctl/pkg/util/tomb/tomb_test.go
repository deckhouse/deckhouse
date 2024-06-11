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

package tomb

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	testReadyEnv    = "READY_MSG"
	testWorkEnv     = "WORK_MSG"
	testShutdownEnv = "SHUTDOWN_MSG"
	testExitCodeEnv = "EXIT_CODE"
)

type testRunResult struct {
	code int
	out  []string
	err  error
}

type testActionParams struct {
	ready, work, shutdown string
	signalAction          func(*exec.Cmd)
	exitCode              int
}

func runAction(p *testActionParams) *testRunResult {
	//nolint:gosec
	cmd := exec.Command(os.Args[0], "-test.run=TestAction")

	readyEnv := fmt.Sprintf("%s=%s", testReadyEnv, p.ready)
	workEnv := fmt.Sprintf("%s=%s", testWorkEnv, p.work)
	shtEnv := fmt.Sprintf("%s=%s", testShutdownEnv, p.shutdown)
	exitCodeEnv := fmt.Sprintf("%s=%v", testExitCodeEnv, p.exitCode)
	cmd.Env = append(os.Environ(), readyEnv, workEnv, shtEnv, exitCodeEnv)

	var b bytes.Buffer
	cmd.Stdout = &b
	cmd.Stderr = &b

	err := cmd.Start()
	if err != nil {
		return &testRunResult{err: fmt.Errorf("cannot start test")}
	}

	go p.signalAction(cmd)

	err = cmd.Wait()

	exitCode := 0
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		exitCode = exitError.ExitCode()
	}

	dirtyLines := strings.Split(b.String(), "\n")
	i := len(dirtyLines) - 1
	for ; i >= 0; i-- {
		l := dirtyLines[i]
		if l == "" || l == "PASS" {
			continue
		}

		break
	}

	lines := make([]string, i+1)
	for j := 0; j <= i; j++ {
		lines[j] = dirtyLines[j]
	}

	fmt.Println(dirtyLines, i, lines)

	return &testRunResult{
		out:  lines,
		code: exitCode,
		err:  nil,
	}
}

func assertRunResult(t *testing.T, res, expected *testRunResult) {
	if res.err != nil {
		t.Fatalf("running error %v", res.err)
	}

	if res.code != expected.code {
		t.Fatalf("incorrect exit code. Need=%v, got=%v", expected.code, res.code)
	}

	if len(res.out) != len(expected.out) {
		t.Fatalf("incorrect outputs len. Need=%v, got=%v", len(expected.out), len(res.out))
	}

	for i, e := range expected.out {
		r := res.out[i]
		if e != r {
			t.Fatalf("incorrect output line %v. Need=%v, got=%v", i, e, r)
		}
	}
}

//nolint:unparam
func sendSignalAction(t *testing.T, s os.Signal, wait time.Duration) func(cmd *exec.Cmd) {
	return func(cmd *exec.Cmd) {
		if wait > 0 {
			time.Sleep(wait)
		}
		err := cmd.Process.Signal(s)
		if err != nil {
			t.Fatalf("cannot send signal %v to process %v: %v", s, cmd.Process.Pid, err)
		}
	}
}

func TestAction(t *testing.T) {
	ready := os.Getenv(testReadyEnv)
	work := os.Getenv(testWorkEnv)
	shutdown := os.Getenv(testShutdownEnv)

	if ready == "" || work == "" || shutdown == "" {
		return
	}

	exitWithCodeStr := os.Getenv(testExitCodeEnv)
	exitWithCode, err := strconv.Atoi(exitWithCodeStr)
	if err != nil {
		t.Fatalf("incorrect exit code %s: %v", exitWithCodeStr, err)
	}

	go WaitForProcessInterruption()

	go func() {
		time.Sleep(1 * time.Second)
		fmt.Printf("%s\n", ready)

		RegisterOnShutdown("Test shutdown", func() {
			time.Sleep(1 * time.Second)
			fmt.Printf("%s\n", shutdown)
		})

		time.Sleep(2 * time.Second)
		fmt.Printf("%s\n", work)

		time.Sleep(1 * time.Second)

		Shutdown(exitWithCode)
	}()

	exitCode := WaitShutdown()
	os.Exit(exitCode)
}

func TestTomb(t *testing.T) {
	ready := "ready"
	work := "work"
	shutdown := "shutdown"

	cases := []struct {
		caseName     string
		params       *testActionParams
		expectedCode int
		expectedOut  []string
	}{
		{
			caseName: "Normal running exit with zero code and running shutdown",
			params: &testActionParams{
				ready:        ready,
				work:         work,
				shutdown:     shutdown,
				signalAction: func(cmd *exec.Cmd) {},
			},
			expectedCode: 0,
			expectedOut:  []string{ready, work, shutdown},
		},

		{
			caseName: "Send SIGTERM exit with zero code and running shutdown",
			params: &testActionParams{
				ready:        ready,
				work:         work,
				shutdown:     shutdown,
				signalAction: sendSignalAction(t, syscall.SIGTERM, 1500*time.Millisecond),
			},
			expectedCode: 0,
			expectedOut: []string{
				ready,
				shutdown,
				`Graceful shutdown by "terminated" signal ...`,
			},
		},

		{
			caseName: "Send SIGINT exit with zero code and running shutdown",
			params: &testActionParams{
				ready:        ready,
				work:         work,
				shutdown:     shutdown,
				signalAction: sendSignalAction(t, syscall.SIGINT, 1500*time.Millisecond),
			},
			expectedCode: 0,
			expectedOut: []string{
				ready,
				shutdown,
				`Graceful shutdown by "interrupt" signal ...`,
			},
		},

		{
			caseName: "Send SIGUSR1 exit with 1 code and running shutdown",
			params: &testActionParams{
				ready:        ready,
				work:         work,
				shutdown:     shutdown,
				signalAction: sendSignalAction(t, syscall.SIGUSR1, 1500*time.Millisecond),
			},
			expectedCode: 1,
			expectedOut: []string{
				ready,
				shutdown,
				`Graceful shutdown by "user defined signal 1" signal ...`,
			},
		},

		{
			caseName: "Send another sig (exclude USR1,TERM,INT) should skipped (code 0 with shutdown)",
			params: &testActionParams{
				ready:        ready,
				work:         work,
				shutdown:     shutdown,
				signalAction: sendSignalAction(t, syscall.SIGUSR2, 1500*time.Millisecond),
			},
			expectedCode: 0,
			expectedOut:  []string{ready, work, shutdown},
		},
	}

	for _, c := range cases {
		t.Run(c.caseName, func(t *testing.T) {
			res := runAction(c.params)
			assertRunResult(t, res, &testRunResult{
				code: c.expectedCode,
				out:  c.expectedOut,
			})
		})
	}
}
