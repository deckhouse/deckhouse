// Copyright 2022 Flant JSC
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

package bootstrap

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
)

type PostBootstrapScriptExecutor struct {
	path      string
	timeout   time.Duration
	sshClient *ssh.Client
	state     *State
}

func NewPostBootstrapScriptExecutor(sshClient *ssh.Client, path string, state *State) *PostBootstrapScriptExecutor {
	return &PostBootstrapScriptExecutor{
		path:      path,
		sshClient: sshClient,
		state:     state,
	}
}

func (e *PostBootstrapScriptExecutor) WithTimeout(timeout time.Duration) *PostBootstrapScriptExecutor {
	e.timeout = timeout
	return e
}

func (e *PostBootstrapScriptExecutor) Execute() error {
	return log.Process("bootstrap", "Execute post-bootstrap script", func() error {
		var err error
		resultToSetState, err := e.run()

		if err != nil {
			msg := fmt.Sprintf("Post execution script was failed: %v", err)
			return errors.New(msg)
		}

		err = e.state.SavePostBootstrapScriptResult(resultToSetState)
		if err != nil {
			log.ErrorF("Post bootstrap script result was not saved: %v", err)
		}

		return nil
	})
}

var resultPattern = regexp.MustCompile("(?m)^Result of post-bootstrap script:(.+)$")

func (e *PostBootstrapScriptExecutor) run() (string, error) {
	var result string
	cmd := e.sshClient.UploadScript(e.path).
		WithTimeout(e.timeout).
		Sudo()

	out, err := cmd.Execute()

	outStr := string(out)
	log.InfoLn(outStr)

	if err != nil {
		return "", fmt.Errorf("run %s: %w", e.path, err)
	}

	submatches := resultPattern.FindAllStringSubmatch(outStr, -1)
	if len(submatches) > 0 && len(submatches[0]) > 1 {
		result = submatches[0][1]
	}

	return result, nil
}

func ValidateScriptFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("Cannot get stats for path %s: %v", path, err)
	}

	mode := info.Mode()

	if !mode.IsRegular() {
		return fmt.Errorf("Post bootstrap script should be regular file")
	}

	perm := info.Mode().Perm()

	if perm&0111 != 0111 || perm&0444 != 0444 {
		return fmt.Errorf("Post bootstrap script should be readable and executable for user group and other (-r-xr-xr-x)")
	}

	return nil
}
