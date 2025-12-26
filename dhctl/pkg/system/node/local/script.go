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

package local

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type Script struct {
	scriptPath        string
	args              []string
	env               map[string]string
	sudo              bool
	stdoutLineHandler func(line string)
	timeout           time.Duration
	cleanupAfterRun   bool
}

func NewScript(path string, args ...string) *Script {
	return &Script{
		scriptPath: path,
		args:       args,
	}
}

func (s *Script) Execute(ctx context.Context) (stdout []byte, err error) {
	cmd := NewCommand(s.scriptPath, s.args...)
	if s.sudo {
		cmd.Sudo(ctx)
	}

	if s.timeout > 0 {
		cmd.WithTimeout(s.timeout)
	}
	if s.env != nil {
		cmd.WithEnv(s.env)
	}
	if s.stdoutLineHandler != nil {
		cmd.WithStdoutHandler(s.stdoutLineHandler)
	}

	if s.cleanupAfterRun {
		defer os.Remove(cmd.program)
	}

	err = cmd.Run(ctx)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitErr.Stderr = cmd.StderrBytes()
		}

		err = fmt.Errorf("execute locally: %w", err)
	}

	return cmd.StdoutBytes(), nil
}

func (s *Script) ExecuteBundle(ctx context.Context, parentDir, bundleDir string) (stdout []byte, err error) {
	srcPath := filepath.Join(parentDir, bundleDir)
	dstPath := filepath.Join("/var/lib/", bundleDir)
	_ = os.RemoveAll(dstPath) // Cleanup from previous runs
	if err = copyRecursively(srcPath, dstPath); err != nil {
		return nil, fmt.Errorf("copy bundle to /var/lib/%s: %w", bundleDir, err)
	}

	cmd := NewCommand(filepath.Join("/var/lib", bundleDir, s.scriptPath), s.args...)
	if s.timeout > 0 {
		cmd.WithTimeout(s.timeout)
	}
	if s.env != nil {
		cmd.WithEnv(s.env)
	}
	if s.stdoutLineHandler != nil {
		cmd.WithStdoutHandler(s.stdoutLineHandler)
	}
	if s.sudo {
		cmd.Sudo(ctx)
	}

	if err = cmd.Run(ctx); err != nil {
		log.DebugF("stdout: %s\n\nstderr: %s\n", cmd.StdoutBytes(), cmd.StderrBytes())
		return nil, fmt.Errorf("execute bundle: %w", err)
	}

	return cmd.StdoutBytes(), nil
}

func (s *Script) Sudo() {
	s.sudo = true
}

func (s *Script) WithStdoutHandler(handler func(string)) {
	s.stdoutLineHandler = handler
}

func (s *Script) WithTimeout(timeout time.Duration) {
	s.timeout = timeout
}

func (s *Script) WithEnvs(envs map[string]string) {
	s.env = envs
}

func (s *Script) WithCleanupAfterExec(doCleanup bool) {
	s.cleanupAfterRun = doCleanup
}

func (s *Script) WithCommanderMode(bool) {}

func (s *Script) WithExecuteUploadDir(string) {}
