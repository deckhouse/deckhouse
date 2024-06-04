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

package terraform

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"
	"syscall"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

// based on https://stackoverflow.com/a/43931246
// https://regex101.com/r/qtIrSj/1
var terraformLogsMatcher = regexp.MustCompile(`(\s+\[(TRACE|DEBUG|INFO|WARN|ERROR)\]\s+|Use TF_LOG=TRACE|there is no package|\-\-\-\-)`)

type Executor interface {
	Output(...string) ([]byte, error)
	Exec(...string) (int, error)
	Stop()
}

func terraformCmd(args ...string) *exec.Cmd {
	cmd := exec.Command("terraform", args...)
	cmd.Env = append(
		os.Environ(),
		"PATH=$PWD/bin:$PATH",
		"TF_IN_AUTOMATION=yes",
		"TF_DATA_DIR="+filepath.Join(app.TmpDirName, "tf_dhctl"),
		"TF_PLUGIN_CACHE_DIR=$PWD/plugins",
	)

	// always use dug log for write its to debug log file
	cmd.Env = append(cmd.Env, "TF_LOG=DEBUG")

	cmd.Env = append(
		cmd.Env,
		fmt.Sprintf("HTTP_PROXY=%s", os.Getenv("HTTP_PROXY")),
		fmt.Sprintf("HTTPS_PROXY=%s", os.Getenv("HTTPS_PROXY")),
		fmt.Sprintf("NO_PROXY=%s", os.Getenv("NO_PROXY")),
	)
	return cmd
}

// CMDExecutor straightforward cmd executor which provides convenient output and handles quit signal.
type CMDExecutor struct {
	cmd *exec.Cmd
}

func (c *CMDExecutor) Output(args ...string) ([]byte, error) {
	return terraformCmd(args...).Output()
}

func (c *CMDExecutor) Exec(args ...string) (int, error) {
	c.cmd = terraformCmd(args...)

	// Start terraform as a leader of the new process group to prevent
	// os.Interrupt (SIGINT) signal from the shell when Ctrl-C is pressed.
	c.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return 1, fmt.Errorf("stdout pipe: %v", err)
	}

	stderr, err := c.cmd.StderrPipe()
	if err != nil {
		return 1, fmt.Errorf("stderr pipe: %v", err)
	}

	log.DebugLn(c.cmd.String())
	err = c.cmd.Start()
	if err != nil {
		log.ErrorLn(err)
		return c.cmd.ProcessState.ExitCode(), err
	}

	var (
		wg     sync.WaitGroup
		errBuf bytes.Buffer
	)

	wg.Add(2)

	go func() {
		defer wg.Done()

		e := bufio.NewScanner(stderr)
		for e.Scan() {
			txt := e.Text()
			log.DebugLn(txt)

			if !app.IsDebug {
				if !terraformLogsMatcher.MatchString(txt) {
					errBuf.WriteString(txt + "\n")
				}
			}
		}
	}()

	go func() {
		defer wg.Done()

		s := bufio.NewScanner(stdout)
		for s.Scan() {
			log.InfoLn(s.Text())
		}
	}()

	wg.Wait()

	err = c.cmd.Wait()

	exitCode := c.cmd.ProcessState.ExitCode() // 2 = exit code, if terraform plan has diff
	if err != nil && exitCode != terraformHasChangesExitCode {
		log.ErrorLn(err)
		err = fmt.Errorf(errBuf.String())
		if app.IsDebug {
			err = fmt.Errorf("terraform has failed in DEBUG mode, search in the output above for an error")
		}
	}

	if exitCode == 0 {
		err = nil
	}
	return exitCode, err
}

func (c *CMDExecutor) Stop() {
	log.DebugF("Interrupt terraform process by pid: %d\n", c.cmd.Process.Pid)

	// 1. Terraform exits immediately on SIGTERM, so SIGINT is used here
	//    to interrupt it gracefully even when main process caught the SIGTERM.
	// 2. Negative pid is used to send signal to the process group
	//    started by "Setpgid: true" to prevent double signaling
	//    from shell and from us.
	//    See also pkg/system/ssh/cmd/ssh.go
	_ = syscall.Kill(-c.cmd.Process.Pid, syscall.SIGINT)
}

// fakeResponse returns data by the first terraform command line argument.
type fakeResponse struct {
	err  error
	code int
	resp []byte
}
type fakeExecutor struct {
	data map[string]fakeResponse
}

func (f *fakeExecutor) Output(parts ...string) ([]byte, error) {
	result := f.data[parts[0]]
	return result.resp, result.err
}
func (f *fakeExecutor) Exec(parts ...string) (int, error) {
	result := f.data[parts[0]]
	return result.code, result.err
}
func (f *fakeExecutor) Stop() {}
