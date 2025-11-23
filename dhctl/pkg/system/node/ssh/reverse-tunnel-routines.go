// Copyright 2024 Flant JSC
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

package ssh

import (
	"context"

	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
)

type RunScriptReverseTunnelChecker struct {
	client     node.SSHClient
	scriptPath string

	uploadDir string
	cleanup   bool
}

func NewRunScriptReverseTunnelChecker(c node.SSHClient, scriptPath string) *RunScriptReverseTunnelChecker {
	return &RunScriptReverseTunnelChecker{
		client:     c,
		scriptPath: scriptPath,
		cleanup:    false,
	}
}

func (s *RunScriptReverseTunnelChecker) WithUploadDir(dir string) *RunScriptReverseTunnelChecker {
	s.uploadDir = dir
	return s
}

func (s *RunScriptReverseTunnelChecker) WithCleanup() *RunScriptReverseTunnelChecker {
	s.cleanup = true
	return s
}

func (s *RunScriptReverseTunnelChecker) CheckTunnel(ctx context.Context) (string, error) {
	script := s.client.UploadScript(s.scriptPath)

	script.Sudo()

	if s.uploadDir != "" {
		script.WithExecuteUploadDir(s.uploadDir)
	}

	if s.cleanup {
		script.WithCleanupAfterExec(s.cleanup)
	}

	out, err := script.Execute(ctx)
	return string(out), err
}

type RunScriptReverseTunnelKiller struct {
	client     node.SSHClient
	scriptPath string
}

func NewRunScriptReverseTunnelKiller(c node.SSHClient, scriptPath string) *RunScriptReverseTunnelKiller {
	return &RunScriptReverseTunnelKiller{
		client:     c,
		scriptPath: scriptPath,
	}
}

func (s *RunScriptReverseTunnelKiller) KillTunnel(ctx context.Context) (string, error) {
	script := s.client.UploadScript(s.scriptPath)
	script.Sudo()
	out, err := script.Execute(ctx)
	return string(out), err
}

type EmptyReverseTunnelKiller struct{}

func (k EmptyReverseTunnelKiller) KillTunnel(ctx context.Context) (string, error) {
	return "", nil
}
