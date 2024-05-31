// Copyright 2021 Flant JSC
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

package frontend

import (
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
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

type UploadScript struct {
	Session *session.Session

	ScriptPath string
	Args       []string
	envs       map[string]string

	sudo bool

	stdoutHandler func(string)

	timeout time.Duration
}

func NewUploadScript(sess *session.Session, scriptPath string, args ...string) *UploadScript {
	return &UploadScript{
		Session:    sess,
		ScriptPath: scriptPath,
		Args:       args,
	}
}

func (u *UploadScript) Sudo() *UploadScript {
	u.sudo = true
	return u
}

func (u *UploadScript) WithStdoutHandler(handler func(string)) *UploadScript {
	u.stdoutHandler = handler
	return u
}

func (u *UploadScript) WithTimeout(timeout time.Duration) *UploadScript {
	u.timeout = timeout
	return u
}

func (u *UploadScript) WithEnvs(envs map[string]string) *UploadScript {
	u.envs = envs
	return u
}

func (u *UploadScript) Execute() (stdout []byte, err error) {
	scriptName := filepath.Base(u.ScriptPath)

	remotePath := "."
	if u.sudo {
		remotePath = filepath.Join(app.DeckhouseNodeTmpPath, scriptName)
	}
	err = NewFile(u.Session).Upload(u.ScriptPath, remotePath)
	if err != nil {
		return nil, fmt.Errorf("upload: %v", err)
	}

	var cmd *Command
	var scriptFullPath string
	if u.sudo {
		scriptFullPath = u.pathWithEnv(filepath.Join(app.DeckhouseNodeTmpPath, scriptName))
		cmd = NewCommand(u.Session, scriptFullPath, u.Args...).Sudo()
	} else {
		scriptFullPath = u.pathWithEnv("./" + scriptName)
		cmd = NewCommand(u.Session, scriptFullPath, u.Args...).Cmd()
	}

	scriptCmd := cmd.CaptureStdout(nil)
	if u.stdoutHandler != nil {
		scriptCmd = scriptCmd.WithStdoutHandler(u.stdoutHandler)
	}

	if u.timeout > 0 {
		scriptCmd.WithTimeout(u.timeout)
	}

	err = scriptCmd.Run()
	if err != nil {
		err = fmt.Errorf("execute on remote: %w", err)
	}
	return cmd.StdoutBytes(), err
}

func (u *UploadScript) pathWithEnv(path string) string {
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

func (u *UploadScript) ExecuteBundle(parentDir, bundleDir string) (stdout []byte, err error) {
	bundleName := fmt.Sprintf("bundle-%s.tar", time.Now().Format("20060102-150405"))
	bundleLocalFilepath := filepath.Join(app.TmpDirName, bundleName)

	// tar cpf bundle.tar -C /tmp/dhctl.1231qd23/var/lib bashible
	tarCmd := exec.Command("tar", "cpf", bundleLocalFilepath, "-C", parentDir, bundleDir)
	err = tarCmd.Run()
	if err != nil {
		return nil, fmt.Errorf("tar bundle: %v", err)
	}

	tomb.RegisterOnShutdown(
		"Delete bashible bundle folder",
		func() { _ = os.Remove(bundleLocalFilepath) },
	)

	// upload to node's deckhouse tmp directory
	err = NewFile(u.Session).Upload(bundleLocalFilepath, app.DeckhouseNodeTmpPath)
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
	bundleCmd := NewCommand(u.Session, tarCmdline).Sudo()

	// Buffers to implement output handler logic
	lastStep := ""
	failsCounter := 0

	processLogger := log.GetProcessLogger()

	handler := bundleOutputHandler(bundleCmd, processLogger, &lastStep, &failsCounter)
	err = bundleCmd.WithStdoutHandler(handler).CaptureStdout(nil).Run()
	if err != nil {
		if lastStep != "" {
			processLogger.LogProcessFail()
		}
		err = fmt.Errorf("execute bundle: %v", err)
	} else {
		processLogger.LogProcessEnd()
	}
	return bundleCmd.StdoutBytes(), err
}

var stepHeaderRegexp = regexp.MustCompile("^=== Step: /var/lib/bashible/bundle_steps/(.*)$")

func bundleOutputHandler(
	cmd *Command,
	processLogger log.ProcessLogger,
	lastStep *string,
	failsCounter *int,
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
				log.ErrorF(strings.Join(stepLogs, "\n"))
				*failsCounter++
				if *failsCounter > 10 {
					if cmd != nil {
						// Force kill bashible
						_ = cmd.cmd.Process.Kill()
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
