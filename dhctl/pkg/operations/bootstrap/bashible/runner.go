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

package bashible

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/deckhouse/lib-dhctl/pkg/log"
	retry "github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/clissh/frontend"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/gossh"
)

const (
	endPipelineFileMark = app.NodeDeckhouseDirectoryPath + "/first-control-plane-bashible-ran"
)

var (
	alreadyRunDefaultOpts    = retry.AttemptsWithWaitOpts(30, 10*time.Second)
	prepareDefaultOpts       = retry.AttemptsWithWaitOpts(30, 10*time.Second)
	executeBundleDefaultOpts = retry.AttemptsWithWaitOpts(10, 10*time.Second)
)

type LoopsParams struct {
	AlreadyRun    retry.Params
	Prepare       retry.Params
	ExecuteBundle retry.Params
}

type Runner struct {
	loggerProvider log.LoggerProvider
	nodeInterface  node.Interface
	loopsParams    LoopsParams
}

func NewRunner(nodeInterface node.Interface, loggerProvider log.LoggerProvider) *Runner {
	return &Runner{
		nodeInterface:  nodeInterface,
		loggerProvider: loggerProvider,
	}
}

func (r *Runner) WithLoopParams(p LoopsParams) *Runner {
	r.loopsParams = p
	return r
}

func (r *Runner) Prepare(ctx context.Context) error {
	if err := r.createDir(ctx, app.NodeDeckhouseDirectoryPath, "0755"); err != nil {
		return err
	}

	if err := r.createDir(ctx, app.DeckhouseNodeTmpPath, "1777"); err != nil {
		return err
	}

	// in end of pipeline steps bashible write "OK" to this file
	// we need creating it before because we do not want handle errors from cat
	return r.touchFile(ctx, endPipelineFileMark)
}

func (r *Runner) AlreadyRun(ctx context.Context) (bool, error) {
	loopParams := retry.SafeCloneOrNewParams(r.loopsParams.AlreadyRun, alreadyRunDefaultOpts...).
		Clone(
			retry.WithName("Checking bashible already ran"),
			retry.WithLogger(r.loggerProvider()),
		)

	isReady := false

	err := retry.NewLoopWithParams(loopParams).RunContext(ctx, func() error {
		cmd := r.nodeInterface.Command("cat", endPipelineFileMark)
		cmd.Sudo(ctx)
		cmd.WithTimeout(10 * time.Second)
		stdout, stderr, err := cmd.Output(ctx)
		if err != nil {
			return err
		}

		r.loggerProvider().DebugF("cat %s stdout: '%s'; stderr: '%s'\n", endPipelineFileMark, stdout, stderr)

		isReady = strings.Contains(string(stdout), "OK")

		return nil
	})

	if err != nil {
		return false, err
	}

	return isReady, nil
}

type ExecuteBundleParams struct {
	BundleDir     string
	CommanderMode bool
}

func (r *Runner) ExecuteBundle(ctx context.Context, params ExecuteBundleParams) error {
	loopParams := retry.SafeCloneOrNewParams(r.loopsParams.ExecuteBundle, executeBundleDefaultOpts...).
		Clone(
			retry.WithName("Execute bundle"),
			retry.WithLogger(r.loggerProvider()),
		)

	return retry.NewLoopWithParams(loopParams).
		BreakIf(bundleTimeoutBreakPredicate).
		RunContext(ctx, func() error {
			// we do not need to restart tunnel because we have HealthMonitor
			logger := r.loggerProvider()

			logger.DebugF("Stop bashible if need")

			if err := r.CleanupPreviousBashibleRunIfNeed(ctx); err != nil {
				return err
			}

			logger.DebugF("Start execute bashible bundle routine")

			return r.attemptExecuteBundle(ctx, params)
		})
}

func (r *Runner) attemptExecuteBundle(ctx context.Context, params ExecuteBundleParams) error {
	bundleCmd := r.nodeInterface.UploadScript("bashible.sh", "--local")
	bundleCmd.WithCommanderMode(params.CommanderMode)
	bundleCmd.WithCleanupAfterExec(false)
	bundleCmd.Sudo()
	parentDir := params.BundleDir + "/var/lib"
	bundleDir := "bashible"

	_, err := bundleCmd.ExecuteBundle(ctx, parentDir, bundleDir)
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return fmt.Errorf("bundle '%s' error: %v\nstderr: %s", bundleDir, err, string(ee.Stderr))
		}

		if errors.Is(err, frontend.ErrBashibleTimeout) {
			return frontend.ErrBashibleTimeout
		}

		if errors.Is(err, gossh.ErrBashibleTimeout) {
			return gossh.ErrBashibleTimeout
		}

		return fmt.Errorf("bundle '%s' error: %v", bundleDir, err)
	}
	return nil
}

func (r *Runner) CleanupPreviousBashibleRunIfNeed(ctx context.Context) error {
	logger := r.loggerProvider()
	return logger.Process("bootstrap", "Cleanup previous bashible run if need", func() error {
		logger.DebugF("Gettting bashible pids")
		pids, err := r.getBashiblePIDs(ctx)
		if err != nil {
			return err
		}

		logger.DebugLn("Got bashible pids: %v", pids)
		if len(pids) == 0 {
			logger.InfoF("Bashible instance not found. Start it!")
			return nil
		}

		if err := r.killBashible(ctx, pids); err != nil {
			return err
		}

		return r.unlockBashible(ctx)
	})
}

func (r *Runner) getBashiblePIDs(ctx context.Context) ([]string, error) {
	logger := r.loggerProvider()

	var psStrings []string
	h := func(l string) {
		psStrings = append(psStrings, l)
	}

	bashCmd := `ps a --no-headers -o args:64 -o "|%p"`
	cmd := r.nodeInterface.Command("bash", "-c", bashCmd)
	cmd.WithStdoutHandler(h)
	if err := r.runCmd(ctx, cmd, bashCmd); err != nil {
		return nil, err
	}

	var res []string
	for _, l := range psStrings {
		logger.DebugF("ps string: '%s'\n", l)

		parts := strings.SplitN(l, "|", 2)
		if len(parts) < 2 {
			logger.DebugLn("Skip ps string without pid")
			continue
		}

		if !strings.Contains(parts[0], "bashible.sh") {
			continue
		}

		pid := strings.TrimSpace(parts[1])
		logger.DebugF("Found bashible PID: %s\n", pid)

		res = append(res, pid)
	}

	return res, nil
}

func (r *Runner) killBashible(ctx context.Context, pids []string) error {
	cmd := r.nodeInterface.Command("kill", pids...)
	return r.runCmd(ctx, cmd, "kill"+strings.Join(pids, " "))
}

func (r *Runner) runCmd(ctx context.Context, cmd node.Command, desc string) error {
	logger := r.loggerProvider()
	cmd.Sudo(ctx)
	cmd.WithTimeout(10 * time.Second)
	if err := cmd.Run(ctx); err != nil {
		var ee *exec.ExitError
		// ssh exits with the exit status of the remote command or with 255 if an error occurred.
		if errors.As(err, &ee) {
			logger.DebugF("'%s' got exit code: %d and stderr %s", desc, ee.ExitCode(), string(ee.Stderr))
			if ee.ExitCode() == 255 {
				return err
			}

			return nil
		}
	}

	return nil
}

func (r *Runner) unlockBashible(ctx context.Context) error {
	cmd := r.nodeInterface.Command("rm", "-f", "/var/lock/bashible")
	return r.runCmd(ctx, cmd, "remove lock file")
}

func (r *Runner) createDir(ctx context.Context, dir, access string) error {
	loopParams := r.prepareLoopParams(dir)

	bashCmd := withUmask("mkdir -p -m %s %s", access, dir)

	err := retry.NewLoopWithParams(loopParams).RunContext(ctx, func() error {
		return r.runWithSH(ctx, bashCmd)
	})

	if err != nil {
		return fmt.Errorf("Cannot create %s directory: %w", dir, err)
	}

	return nil
}

func (r *Runner) touchFile(ctx context.Context, file string) error {
	loopParams := r.prepareLoopParams(file)

	bashCmd := withUmask("touch %s", file)

	err := retry.NewLoopWithParams(loopParams).RunContext(ctx, func() error {
		return r.runWithSH(ctx, bashCmd)
	})

	if err != nil {
		return fmt.Errorf("Cannot touch %s file: %w", file, err)
	}

	return nil
}

func (r *Runner) runWithSH(ctx context.Context, bashCmd string) error {
	cmd := r.nodeInterface.Command("sh", "-c", bashCmd)
	cmd.Sudo(ctx)
	cmd.WithTimeout(10 * time.Second)
	if err := cmd.Run(ctx); err != nil {
		return fmt.Errorf("ssh: %s: %w", bashCmd, err)
	}

	return nil
}

func (r *Runner) prepareLoopParams(target string) retry.Params {
	return retry.SafeCloneOrNewParams(r.loopsParams.Prepare, prepareDefaultOpts...).
		Clone(
			retry.WithName("Prepare %s", target),
			retry.WithLogger(r.loggerProvider()),
		)
}

func withUmask(f string, args ...any) string {
	return fmt.Sprintf("umask 0022 ; "+f, args...)
}

func bundleTimeoutBreakPredicate(err error) bool {
	return errors.Is(err, frontend.ErrBashibleTimeout) || errors.Is(err, gossh.ErrBashibleTimeout)
}
