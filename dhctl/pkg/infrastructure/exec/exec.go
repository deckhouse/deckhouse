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
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

const HasChangesExitCode = 2

// based on https://stackoverflow.com/a/43931246
// https://regex101.com/r/qtIrSj/1
var infrastructureLogsMatcher = regexp.MustCompile(`(\s+\[(TRACE|DEBUG|INFO|WARN|ERROR)\]\s+|Use TF_LOG=TRACE|there is no package|\-\-\-\-)`)

func Exec(ctx context.Context, cmd *exec.Cmd, logger log.Logger) (int, error) {
	// Start infrastructure utility as a leader of the new process group to prevent
	// os.Interrupt (SIGINT) signal from the shell when Ctrl-C is pressed.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 1, fmt.Errorf("stdout pipe: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return 1, fmt.Errorf("stderr pipe: %v", err)
	}

	log.DebugLn(cmd.String())
	err = cmd.Start()
	if err != nil {
		log.ErrorF("Cannot start cmd: %v\n", err)
		return cmd.ProcessState.ExitCode(), err
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
				if !infrastructureLogsMatcher.MatchString(txt) {
					errBuf.WriteString(txt + "\n")
				}
			}
		}
	}()

	go func() {
		defer wg.Done()

		s := bufio.NewScanner(stdout)
		for s.Scan() {
			logger.LogInfoLn(s.Text())
		}
	}()

	wg.Wait()

	err = cmd.Wait()

	exitCode := cmd.ProcessState.ExitCode() // 2 = exit code, if infrastructure plan has diff
	if err != nil && exitCode != HasChangesExitCode {
		logger.LogErrorF("Error while process exit code: %v\n", err)
		err = fmt.Errorf("%s", errBuf.String())
		if app.IsDebug {
			err = fmt.Errorf("infrastructure utility has failed in DEBUG mode, search in the output above for an error")
		}
	}

	if exitCode == 0 {
		err = nil
	}
	return exitCode, err
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
