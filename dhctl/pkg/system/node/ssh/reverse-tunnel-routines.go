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

type BaseReverseTunnelRoutines[T any] struct {
	uploadDir string
	cleanup   bool

	impl *T
}

func newBaseReverseTunnel[T any](impl *T) *BaseReverseTunnelRoutines[T] {
	return &BaseReverseTunnelRoutines[T]{
		impl:      impl,
		cleanup:   false,
		uploadDir: "",
	}
}

func (b *BaseReverseTunnelRoutines[T]) WithUploadDir(dir string) *T {
	b.uploadDir = dir
	return b.impl
}

func (b *BaseReverseTunnelRoutines[T]) WithCleanup() *T {
	b.cleanup = true
	return b.impl
}

func (b *BaseReverseTunnelRoutines[T]) SetUploadDirAndCleanup(dir string) *T {
	b.WithUploadDir(dir)
	b.WithCleanup()

	return b.impl
}

func (b *BaseReverseTunnelRoutines[T]) prepareScript(script node.Script) {
	if b.uploadDir != "" {
		script.WithExecuteUploadDir(b.uploadDir)
	}

	if b.cleanup {
		script.WithCleanupAfterExec(b.cleanup)
	}
}

type RunScriptReverseTunnelChecker struct {
	*BaseReverseTunnelRoutines[RunScriptReverseTunnelChecker]

	client     node.SSHClient
	scriptPath string
}

func NewRunScriptReverseTunnelChecker(c node.SSHClient, scriptPath string) *RunScriptReverseTunnelChecker {
	checker := &RunScriptReverseTunnelChecker{
		client:     c,
		scriptPath: scriptPath,
	}

	checker.BaseReverseTunnelRoutines = newBaseReverseTunnel(checker)

	return checker
}

func (s *RunScriptReverseTunnelChecker) CheckTunnel(ctx context.Context) (string, error) {
	script := s.client.UploadScript(s.scriptPath)

	script.Sudo()

	s.prepareScript(script)

	out, err := script.Execute(ctx)
	return string(out), err
}

type RunScriptReverseTunnelKiller struct {
	*BaseReverseTunnelRoutines[RunScriptReverseTunnelKiller]

	client     node.SSHClient
	scriptPath string
}

func NewRunScriptReverseTunnelKiller(c node.SSHClient, scriptPath string) *RunScriptReverseTunnelKiller {
	killer := &RunScriptReverseTunnelKiller{
		client:     c,
		scriptPath: scriptPath,
	}

	killer.BaseReverseTunnelRoutines = newBaseReverseTunnel(killer)

	return killer
}

func (s *RunScriptReverseTunnelKiller) KillTunnel(ctx context.Context) (string, error) {
	script := s.client.UploadScript(s.scriptPath)

	script.Sudo()

	s.prepareScript(script)

	out, err := script.Execute(ctx)
	return string(out), err
}

type EmptyReverseTunnelKiller struct{}

func (k EmptyReverseTunnelKiller) KillTunnel(context.Context) (string, error) {
	return "", nil
}
