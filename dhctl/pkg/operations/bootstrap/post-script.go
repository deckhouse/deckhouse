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
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
)

type PostBootstrapScriptExecutor struct {
	path        string
	timeout     time.Duration
	errorIfFail bool
	sshClient   *ssh.Client
	state       *State
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

func (e *PostBootstrapScriptExecutor) WithErrorIfExecutionFail(f bool) *PostBootstrapScriptExecutor {
	e.errorIfFail = f
	return e
}

func (e *PostBootstrapScriptExecutor) Execute() error {
	var resultToSetState string
	err := log.Process("bootstrap", "Execute post-bootstrap script", func() error {
		var err error
		resultToSetState, err = e.run()

		return err
	})

	if err != nil {
		msg := fmt.Sprintf("Post execution script was failed: %v", err)
		if app.PostBootstrapScriptExitIfFailed {
			return errors.New(msg)
		}

		log.ErrorF(msg)

		return nil
	}

	log.InfoF("Post-bootstrap script result:%s", resultToSetState)

	err = e.state.SavePostBootstrapScriptResult(resultToSetState)
	if err != nil {
		log.ErrorF("Post bootstrap script result was not saved: %v", err)
	}

	return nil
}

func (e *PostBootstrapScriptExecutor) run() (string, error) {
	var stdout []string
	stdoutHandler := func(l string) {
		stdout = append(stdout, l)
	}

	stderrHandler := func(l string) {
		log.InfoLn(l)
	}
	cmd := e.sshClient.UploadScript(e.path).
		WithStdoutHandler(stdoutHandler).
		WithStderrHandler(stderrHandler).
		WithTimeout(e.timeout).
		Sudo()

	_, err := cmd.Execute()
	if err != nil {
		return "", fmt.Errorf("run %s: %w", e.path, err)
	}

	return strings.Join(stdout, "\n"), nil
}
