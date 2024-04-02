/*
Copyright 2023 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ssh

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"caps-controller-manager/internal/scope"
)

// ExecSSHCommand executes a command on the StaticInstance.
func ExecSSHCommand(instanceScope *scope.InstanceScope, command string, stdout io.Writer) error {
	privateSSHKey, err := base64.StdEncoding.DecodeString(instanceScope.Credentials.Spec.PrivateSSHKey)
	if err != nil {
		return errors.Wrap(err, "failed to decode private ssh key")
	}

	dir := os.TempDir()
	sshKey := filepath.Join(dir, "ssh-key")

	err = os.WriteFile(sshKey, privateSSHKey, 0600)
	if err != nil {
		return errors.Wrap(err, "failed to write private ssh key to temporary file")
	}

	args := []string{
		"-qv",
		"-i",
		sshKey,
		"-o",
		"StrictHostKeyChecking=no",
		fmt.Sprintf("-p %d", instanceScope.Credentials.Spec.SSHPort),
	}

	var stdin io.Reader

	// If the sudo password is set, we need to pipe it to the ssh command.
	if instanceScope.Credentials.Spec.SudoPassword != "" {
		stdin = bytes.NewBufferString(instanceScope.Credentials.Spec.SudoPassword)

		command = fmt.Sprintf(`sudo -S sh -c "%s"`, command)
	} else {
		args = append(args, "-o", "PasswordAuthentication=no")

		command = fmt.Sprintf(`sudo sh -c "%s"`, command)
	}

	for _, arg := range strings.Split(instanceScope.Credentials.Spec.SSHExtraArgs, " ") {
		if arg == "" {
			continue
		}

		args = append(args, arg)
	}

	args = append(args, []string{
		fmt.Sprintf("%s@%s", instanceScope.Credentials.Spec.User, instanceScope.Instance.Spec.Address),
		command,
	}...)

	cmd := exec.Command("ssh", args...)

	cmd.Stdin = stdin

	if stdout == nil {
		stdout = NewLogger(instanceScope.Logger.WithName("stdout"))
	}

	cmd.Stdout = stdout
	cmd.Stderr = NewLogger(instanceScope.Logger.WithName("stderr"))

	instanceScope.Logger.Info("Exec ssh command", "command", cmd.String())

	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "failed to run ssh command")
	}

	return nil
}

// ExecSSHCommandToString executes a command on the StaticInstance and returns the output as a string.
func ExecSSHCommandToString(instanceScope *scope.InstanceScope, command string) (string, error) {
	stdout := &bytes.Buffer{}

	err := ExecSSHCommand(instanceScope, command, stdout)
	if err != nil {
		return "", errors.Wrap(err, "failed to exec ssh command")
	}

	stdoutBytes, err := io.ReadAll(stdout)
	if err != nil {
		return "", errors.Wrap(err, "failed to read stdout from ssh command")
	}

	return strings.TrimSpace(string(stdoutBytes)), nil
}
