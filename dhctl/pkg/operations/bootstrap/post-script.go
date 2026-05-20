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

package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
)

type PostBootstrapScriptExecutor struct {
	path                   string
	timeout                time.Duration
	sshProviderinitializer *providerinitializer.SSHProviderInitializer
	state                  *State
}

func NewPostBootstrapScriptExecutor(sshProviderinitializer *providerinitializer.SSHProviderInitializer, path string, state *State) *PostBootstrapScriptExecutor {
	return &PostBootstrapScriptExecutor{
		path:                   path,
		sshProviderinitializer: sshProviderinitializer,
		state:                  state,
	}
}

func (e *PostBootstrapScriptExecutor) WithTimeout(timeout time.Duration) *PostBootstrapScriptExecutor {
	e.timeout = timeout
	return e
}

func (e *PostBootstrapScriptExecutor) Execute(ctx context.Context) error {
	return log.ProcessCtx(ctx, "bootstrap", "Execute post-bootstrap script", func(ctx context.Context) error {
		resultToSetState, err := e.run(ctx)
		if err != nil {
			msg := fmt.Sprintf("Post execution script was failed: %v", err)
			return errors.New(msg)
		}

		if err := e.state.SavePostBootstrapScriptResult(ctx, resultToSetState); err != nil {
			log.ErrorF("Post bootstrap script result was not saved: %v", err)
		}

		return nil
	})
}

func (e *PostBootstrapScriptExecutor) run(ctx context.Context) (result string, err error) {
	outputFile := fs.RandomNumberSuffix("/tmp/post-bootstrap-script-output")
	envs := map[string]string{
		"OUTPUT": outputFile,
	}

	sshProvider, err := e.sshProviderinitializer.GetSSHProvider(ctx)
	if err != nil {
		return "", err
	}

	sshClient, err := sshProvider.Client(ctx)
	if err != nil {
		return "", err
	}

	createOUtFileCmd := fmt.Sprintf("touch %s && chmod 644 %s", outputFile, outputFile)
	cmd := sshClient.Command(createOUtFileCmd)
	cmd.WithStderrHandler(nil)
	cmd.WithStdoutHandler(nil)
	cmd.Sudo(ctx)

	if err := cmd.Run(ctx); err != nil {
		return "", fmt.Errorf("Cannot create output file for script: %v", err)
	}

	defer func() {
		// remove out file on server because it can contain non-safe information
		cmd = sshClient.Command(fmt.Sprintf("rm %s", outputFile))
		cmd.WithStderrHandler(nil)
		cmd.WithStdoutHandler(nil)
		cmd.Sudo(ctx)
		err = cmd.Run(ctx)
	}()

	script := sshClient.UploadScript(e.path)
	script.WithTimeout(e.timeout)
	script.WithStdoutHandler(func(s string) {
		log.InfoLn(s)
	})
	script.WithEnvs(envs)
	script.Sudo()

	if _, err := script.Execute(ctx); err != nil {
		return "", fmt.Errorf("Running %s done with error: %w", e.path, err)
	}

	content, err := sshClient.File().DownloadBytes(ctx, outputFile)
	if err != nil {
		return "", fmt.Errorf("Cannot get output from remote file %s: %w", e.path, err)
	}

	return string(content), nil
}

func ValidateScriptFile(ctx context.Context, path string) error {
	_, span := telemetry.StartSpan(ctx, "ValidatePostBootstrapScript")
	defer span.End()

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
