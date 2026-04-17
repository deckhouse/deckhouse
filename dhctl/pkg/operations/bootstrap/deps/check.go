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

package deps

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	tplt "text/template"
	"time"

	"github.com/deckhouse/lib-dhctl/pkg/log"
	retry "github.com/deckhouse/lib-dhctl/pkg/retry"
	"github.com/hashicorp/go-multierror"
	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
)

type LoopsParams struct {
	Shell        retry.Params
	Dependencies retry.Params
}

type DependenciesChecker struct {
	loggerProvider log.LoggerProvider
	nodeInterface  node.Interface
	loopsParams    LoopsParams
}

var (
	ErrMissingDeps    = errors.New("Have missing dependencies")
	ErrShellIsNotBash = errors.New(
		"Bashible requires /bin/bash as the user's login shell. Please change the user's shell",
	)

	dependencies = []string{
		"sudo", "rm", "tar", "mount", "awk",
		"grep", "cut", "sed", "mkdir", "cp",
		"join", "cat", "ps", "kill",
	}

	checkDepsDefaultOpts  = retry.AttemptsWithWaitOpts(10, 5*time.Second)
	checkShellDefaultOpts = retry.AttemptsWithWaitOpts(10, 5*time.Second)
)

func NewDependenciesChecker(nodeInterface node.Interface, loggerProvider log.LoggerProvider) *DependenciesChecker {
	return &DependenciesChecker{
		nodeInterface:  nodeInterface,
		loggerProvider: loggerProvider,
	}
}

func (c *DependenciesChecker) WithLoopsParams(p LoopsParams) *DependenciesChecker {
	c.loopsParams = p

	return c
}

func (c *DependenciesChecker) Check(ctx context.Context) error {
	if govalue.IsNil(c.nodeInterface) {
		return fmt.Errorf("Internal error: node is nil for dependencies checker")
	}

	var resErr *multierror.Error

	if err := c.checkShell(ctx); err != nil {
		resErr = multierror.Append(resErr, err)
	}

	if err := c.checkDependencies(ctx); err != nil {
		resErr = multierror.Append(resErr, err)
	}

	return resErr.ErrorOrNil()
}

func (c *DependenciesChecker) checkShell(ctx context.Context) error {
	return c.loggerProvider().Process(log.ProcessBootstrap, "Check user's shell is bash", func() error {
		loopParams := retry.SafeCloneOrNewParams(c.loopsParams.Shell, checkDepsDefaultOpts...).
			Clone(
				retry.WithName("Check shell is bash"),
				retry.WithLogger(c.loggerProvider()),
			)

		err := retry.NewSilentLoopWithParams(loopParams).
			BreakIf(c.shellErrorBreakPredicate).
			RunContext(ctx, func() error {
				cmd := c.nodeInterface.Command("echo $SHELL")
				out, stderr, err := cmd.Output(ctx)
				if err != nil {
					return fmt.Errorf("Error checking shell: %s: %w", string(stderr), err)
				}

				strOut := strings.TrimSpace(string(out))
				if strOut == "" {
					strOut = "not set"
				}

				if !strings.Contains(strOut, "bash") {
					return fmt.Errorf("%w. Current shell: %s", ErrShellIsNotBash, strOut)
				}

				return nil
			})
		if err != nil {
			return err
		}

		c.loggerProvider().InfoF("OK!")
		return nil
	})

}

func (c *DependenciesChecker) checkDependencies(ctx context.Context) error {
	return c.loggerProvider().Process(log.ProcessBootstrap, "Check all DHCTL dependencies", func() error {
		loopParams := retry.SafeCloneOrNewParams(c.loopsParams.Dependencies, checkDepsDefaultOpts...).
			Clone(
				retry.WithName("Check all DHCTL dependencies"),
				retry.WithLogger(c.loggerProvider()),
			)

		runErr := retry.NewSilentLoopWithParams(loopParams).
			BreakIf(c.depsErrorBreakPredicate).
			RunContext(ctx, func() error {
				output, err := c.runBinariesCheckScript(ctx)
				if err != nil {
					return err
				}

				return c.processBinariesCheckResult(output)
			})

		if runErr != nil {
			return runErr
		}

		return nil
	})
}

func (c *DependenciesChecker) processBinariesCheckResult(output []byte) error {
	var missing []string
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			continue
		}
		status, dep := fields[0], fields[1]
		statusCode, err := strconv.Atoi(status)
		if err != nil {
			// Skipping non-numeric output line, hack to bypass problems with the sshd banner
			continue
		}

		logger := c.loggerProvider()

		logger.InfoF("Checking '%s' dependency", dep)
		if statusCode == 1 {
			logger.Success(fmt.Sprintf("Dependency '%s' is available", dep))
		} else {
			c.loggerProvider().WarnF(fmt.Sprintf("Dependency '%s' is missing!", dep))
			missing = append(missing, dep)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("Failed to read dependency output: %w", err)
	}

	if len(missing) > 0 {
		return fmt.Errorf("%w: %s", ErrMissingDeps, strings.Join(missing, ", "))
	}

	return nil
}

func (c *DependenciesChecker) runBinariesCheckScript(ctx context.Context) ([]byte, error) {
	bashScript, err := buildDependencyCheckScript(dependencies)
	if err != nil {
		return nil, fmt.Errorf("failed to build dependency check script: %w", err)
	}

	logger := c.loggerProvider()

	logger.DebugF("Generated dependency check bash script:\n%s\n", bashScript)
	// Encode the script to avoid "\n" characters and safely pass it via SSH
	encoded := base64.StdEncoding.EncodeToString([]byte(bashScript))
	remoteCmd := fmt.Sprintf("echo %q | base64 -d | bash", encoded)

	cmd := c.nodeInterface.Command("bash", "-c", remoteCmd)
	output, err := cmd.CombinedOutput(ctx)
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			logger.DebugF("SSH exit code: %v\n", ee.ExitCode())
		}
		e := fmt.Errorf("remote dependency check failed: %w - %s", err, string(output))
		logger.DebugF("Dependency check error: %v\n", e)
		return nil, e
	}

	return output, nil
}

func (c *DependenciesChecker) depsErrorBreakPredicate(err error) bool {
	if !c.sshErrorBreakPredicate(err) {
		return false
	}

	if errors.Is(err, ErrMissingDeps) {
		c.loggerProvider().DebugF("Has missing deps error. Break cycle")
		return true
	}

	return false
}

func (c *DependenciesChecker) shellErrorBreakPredicate(err error) bool {
	if !c.sshErrorBreakPredicate(err) {
		return false
	}

	if errors.Is(err, ErrShellIsNotBash) {
		c.loggerProvider().DebugF("Has not bash error. Break cycle")
		return true
	}

	return false
}

func (c *DependenciesChecker) sshErrorBreakPredicate(err error) bool {
	// Retry only for transient SSH connection issues
	if err == nil {
		return true
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 255 {
		c.loggerProvider().WarnF("SSH connection failed (exit 255), retrying in 5 seconds...")
		return false
	}

	return true
}

const dependencyCheckTemplate = `
for dep in {{range $i, $d := .Deps}}{{if $i}} {{end}}{{$d}}{{end}}; do
  if command -v "$dep" >/dev/null 2>&1; then
    echo "1 $dep"
  else
    echo "0 $dep"
  fi
done
`

func buildDependencyCheckScript(deps []string) (string, error) {
	tmpl, err := tplt.New("dep-check").Parse(dependencyCheckTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse dependency template: %w", err)
	}

	tplData := map[string]any{
		"Deps": deps,
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, tplData)
	if err != nil {
		return "", fmt.Errorf("failed to render dependency template: %w", err)
	}

	return buf.String(), nil
}
