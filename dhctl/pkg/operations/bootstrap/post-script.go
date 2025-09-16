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
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
)

type PostBootstrapScriptExecutor struct {
	path      string
	timeout   time.Duration
	sshClient node.SSHClient
	state     *State
}

func NewPostBootstrapScriptExecutor(sshClient node.SSHClient, path string, state *State) *PostBootstrapScriptExecutor {
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

func (e *PostBootstrapScriptExecutor) Execute(ctx context.Context) error {
	return log.Process("bootstrap", "Execute post-bootstrap script", func() error {
		var err error
		resultToSetState, err := e.run(ctx)

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

func (e *PostBootstrapScriptExecutor) run(ctx context.Context) (string, error) {
	outputFile := fs.RandomNumberSuffix("/tmp/post-bootstrap-script-output")
	envs := map[string]string{
		"OUTPUT": outputFile,
	}

	createOUtFileCmd := fmt.Sprintf("touch %s && chmod 644 %s", outputFile, outputFile)
	cmd := e.sshClient.Command(createOUtFileCmd)
	cmd.Sudo(ctx)
	cmd.WithStderrHandler(nil)
	cmd.WithStdoutHandler(nil)
	err := cmd.Run(ctx)

	if err != nil {
		return "", fmt.Errorf("Cannot create output file for script: %v", err)
	}

	defer func() {
		// remove out file on server because it can contain non-safe information
		cmd = e.sshClient.Command(fmt.Sprintf("rm %s", outputFile))
		cmd.Sudo(ctx)
		cmd.WithStderrHandler(nil)
		cmd.WithStdoutHandler(nil)
		err = cmd.Run(ctx)
	}()

	script := e.sshClient.UploadScript(e.path)
	script.WithTimeout(e.timeout)
	script.WithStdoutHandler(func(s string) {
		log.InfoLn(s)
	})
	script.WithEnvs(envs)
	script.Sudo()

	_, err = script.Execute(ctx)

	if err != nil {
		return "", fmt.Errorf("Running %s done with error: %w", e.path, err)
	}

	content, err := e.sshClient.File().DownloadBytes(ctx, outputFile)
	if err != nil {
		return "", fmt.Errorf("Cannot get output from remote file %s: %w", e.path, err)
	}

	if err != nil {
		log.WarnLn("Post bootstrap output file '%s' did not remove from server", outputFile)
	}

	return string(content), nil
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
