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

package gossh

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/alessio/shellescape"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tar"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
	"golang.org/x/crypto/ssh"
)

type SSHUploadScript struct {
	sshClient *Client

	ScriptPath string
	Args       []string
	envs       map[string]string

	sudo bool

	cleanupAfterExec bool

	stdoutHandler func(string)

	timeout time.Duration

	commanderMode bool
}

func NewSSHUploadScript(sshClient *Client, scriptPath string, args ...string) *SSHUploadScript {
	return &SSHUploadScript{
		sshClient:  sshClient,
		ScriptPath: scriptPath,
		Args:       args,

		cleanupAfterExec: true,
	}
}

func (u *SSHUploadScript) Sudo() {
	u.sudo = true
}

func (u *SSHUploadScript) WithStdoutHandler(handler func(string)) {
	u.stdoutHandler = handler
}

func (u *SSHUploadScript) WithTimeout(timeout time.Duration) {
	u.timeout = timeout
}

func (u *SSHUploadScript) WithEnvs(envs map[string]string) {
	u.envs = envs
}

func (u *SSHUploadScript) WithCommanderMode(enabled bool) {
	u.commanderMode = enabled
}

// WithCleanupAfterExec option tells if ssh executor should delete uploaded script after execution was attempted or not.
// It does not care if script was executed successfully of failed.
func (u *SSHUploadScript) WithCleanupAfterExec(doCleanup bool) {
	u.cleanupAfterExec = doCleanup
}

func (u *SSHUploadScript) Execute(ctx context.Context) (stdout []byte, err error) {
	scriptName := filepath.Base(u.ScriptPath)

	remotePath := "."
	if u.sudo {
		remotePath = filepath.Join(app.DeckhouseNodeTmpPath, scriptName)
	}
	log.DebugF("Uploading script %s to %s\n", u.ScriptPath, remotePath)
	err = NewSSHFile(u.sshClient.sshClient).Upload(ctx, u.ScriptPath, remotePath)
	if err != nil {
		return nil, fmt.Errorf("upload: %v", err)
	}

	var cmd *SSHCommand
	var scriptFullPath string
	if u.sudo {
		scriptFullPath = u.pathWithEnv(filepath.Join(app.DeckhouseNodeTmpPath, scriptName))
		cmd = NewSSHCommand(u.sshClient, scriptFullPath, u.Args...)
		cmd.Sudo(ctx)
	} else {
		scriptFullPath = u.pathWithEnv("./" + scriptName)
		cmd = NewSSHCommand(u.sshClient, scriptFullPath, u.Args...)
		cmd.Cmd(ctx)
	}

	if u.stdoutHandler != nil {
		cmd.WithStdoutHandler(u.stdoutHandler)
	}

	if u.timeout > 0 {
		cmd.WithTimeout(u.timeout)
	}

	err = cmd.Run(ctx)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// exitErr.Stderr is set in the "os/exec".Cmd.Output method from the Golang standard library.
			// But we call the "os/exec".Cmd.Wait method, which does not set the Stderr field.
			// We can reuse the exec.ExitError type when handling errors.
			exitErr.Stderr = cmd.StderrBytes()
		}

		err = fmt.Errorf("execute on remote: %w", err)
	}

	if u.cleanupAfterExec {
		defer func() {
			err := NewSSHCommand(u.sshClient, "rm", "-f", scriptFullPath).Run(ctx)
			if err != nil {
				log.DebugF("Failed to delete uploaded script %s: %v", scriptFullPath, err)
			}
		}()
	}

	return cmd.StdoutBytes(), err
}

func (u *SSHUploadScript) pathWithEnv(path string) string {
	if len(u.envs) == 0 {
		return path
	}

	arrayToJoin := make([]string, 0, len(u.envs)*2)

	for k, v := range u.envs {
		vEscaped := shellescape.Quote(v)
		kvStr := fmt.Sprintf("%s=%s", k, vEscaped)
		arrayToJoin = append(arrayToJoin, kvStr)
	}

	envs := strings.Join(arrayToJoin, " ")

	return fmt.Sprintf("%s %s", envs, path)
}

var ErrBashibleTimeout = errors.New("Timeout bashible step running")

func (u *SSHUploadScript) ExecuteBundle(ctx context.Context, parentDir, bundleDir string) (stdout []byte, err error) {
	bundleName := fmt.Sprintf("bundle-%s.tar", time.Now().Format("20060102-150405"))
	bundleLocalFilepath := filepath.Join(app.TmpDirName, bundleName)

	// tar cpf bundle.tar -C /tmp/dhctl.1231qd23/var/lib bashible
	err = tar.CreateTar(bundleLocalFilepath, parentDir, bundleDir)
	if err != nil {
		return nil, fmt.Errorf("tar bundle: %v", err)
	}

	tomb.RegisterOnShutdown(
		"Delete bashible bundle folder",
		func() { _ = os.Remove(bundleLocalFilepath) },
	)

	// upload to node's deckhouse tmp directory
	err = NewSSHFile(u.sshClient.sshClient).Upload(ctx, bundleLocalFilepath, app.DeckhouseNodeTmpPath)
	if err != nil {
		return nil, fmt.Errorf("upload: %v", err)
	}

	// sudo:
	// tar xpof ${app.DeckhouseNodeTmpPath}/bundle.tar -C /var/lib && /var/lib/bashible/bashible.sh args...
	tarCmdline := fmt.Sprintf(
		"tar xpof %s/%s -C /var/lib && /var/lib/%s/%s %s",
		app.DeckhouseNodeTmpPath,
		bundleName,
		bundleDir,
		u.ScriptPath,
		strings.Join(u.Args, " "),
	)
	bundleCmd := NewSSHCommand(u.sshClient, tarCmdline)
	bundleCmd.Sudo(ctx)

	// Buffers to implement output handler logic
	lastStep := ""
	failsCounter := 0
	isBashibleTimeout := false

	processLogger := log.GetProcessLogger()

	handler := bundleSSHOutputHandler(bundleCmd, processLogger, &lastStep, &failsCounter, &isBashibleTimeout, u.commanderMode)
	bundleCmd.WithStdoutHandler(handler)
	bundleCmd.CaptureStdout(nil)
	bundleCmd.CaptureStderr(nil)
	err = bundleCmd.Run(ctx)
	if err != nil {
		if lastStep != "" {
			processLogger.LogProcessFail()
		}

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// exitErr.Stderr is set in the "os/exec".Cmd.Output method from the Golang standard library.
			// But we call the "os/exec".Cmd.Wait method, which does not set the Stderr field.
			// We can reuse the exec.ExitError type when handling errors.
			exitErr.Stderr = bundleCmd.StderrBytes()
		}

		err = fmt.Errorf("execute bundle: %w", err)
	} else {
		processLogger.LogProcessEnd()
	}

	if isBashibleTimeout {
		return bundleCmd.StdoutBytes(), ErrBashibleTimeout
	}

	return bundleCmd.StdoutBytes(), err
}

var stepHeaderRegexp = regexp.MustCompile("^=== Step: /var/lib/bashible/bundle_steps/(.*)$")

func bundleSSHOutputHandler(
	cmd *SSHCommand,
	processLogger log.ProcessLogger,
	lastStep *string,
	failsCounter *int,
	isBashibleTimeout *bool,
	commanderMode bool,
) func(string) {
	stepLogs := make([]string, 0)
	return func(l string) {
		if l == "===" {
			return
		}
		if stepHeaderRegexp.Match([]byte(l)) {
			match := stepHeaderRegexp.FindStringSubmatch(l)
			stepName := match[1]

			if *lastStep == stepName {
				logMessage := strings.Join(stepLogs, "\n")
				switch {
				case commanderMode && *failsCounter == 0:
					log.ErrorF("%s", logMessage)
				case commanderMode && *failsCounter > 0:
					log.ErrorF("Run step %s finished with error^^^\n", stepName)
					log.DebugF("%s", logMessage)
				default:
					log.ErrorF("%s", logMessage)
				}
				*failsCounter++
				stepLogs = stepLogs[:0]
				if *failsCounter > 10 {
					*isBashibleTimeout = true
					if cmd != nil {
						// Force kill bashible and close session/streams to unblock Wait/readers
						_ = cmd.session.Signal(ssh.SIGABRT)
						if cmd.Stdin != nil {
							_ = cmd.Stdin.Close()
						}
						_ = cmd.session.Close()
					}
					return
				}

				processLogger.LogProcessFail()
				stepName = fmt.Sprintf("%s, retry attempt #%d of 10", stepName, *failsCounter)
			} else if *lastStep != "" {
				stepLogs = make([]string, 0)
				processLogger.LogProcessEnd()
				*failsCounter = 0
			}

			processLogger.LogProcessStart("Run step " + stepName)
			*lastStep = match[1]
			return
		}

		stepLogs = append(stepLogs, l)
		log.DebugLn(l)
	}
}
