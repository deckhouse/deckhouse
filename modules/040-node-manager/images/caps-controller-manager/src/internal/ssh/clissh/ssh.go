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

package clissh

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"caps-controller-manager/internal/scope"
	genssh "caps-controller-manager/internal/ssh"
)

type SSH struct{}

func CreateSSHClient(instanceScope *scope.InstanceScope) (*SSH, error) {
	return &SSH{}, nil
}

// ExecSSHCommand executes a command on the StaticInstance.
func (s *SSH) ExecSSHCommand(instanceScope *scope.InstanceScope, command string, stdout io.Writer, stderr io.Writer) error {
	privateSSHKey, err := base64.StdEncoding.DecodeString(instanceScope.Credentials.Spec.PrivateSSHKey)
	if err != nil {
		return errors.Wrap(err, "failed to decode private ssh key")
	}

	privateSSHKey = append(bytes.TrimSpace(privateSSHKey), '\n')

	sshKey, err := os.CreateTemp("", "ssh-key-")
	if err != nil {
		return errors.Wrap(err, "failed to create a temporary file for private ssh key")
	}
	defer func() {
		err := sshKey.Close()
		if err != nil {
			// It is not critical if we can't close the file.
			instanceScope.Logger.Error(err, fmt.Sprintf("failed to close temporary file '%s' with private ssh key", sshKey.Name()))
		}

		err = os.Remove(sshKey.Name())
		if err != nil {
			// It is not critical if we can't remove the file.
			instanceScope.Logger.Error(err, fmt.Sprintf("failed to remove temporary file '%s' with private ssh key", sshKey.Name()))
		}
	}()

	_, err = io.Copy(sshKey, bytes.NewReader(privateSSHKey))
	if err != nil {
		return errors.Wrapf(err, "failed to write private ssh key to temporary file '%s'", sshKey.Name())
	}

	args := []string{
		"-qv",
		"-i",
		sshKey.Name(),
		"-o",
		"StrictHostKeyChecking=no",
		fmt.Sprintf("-p %d", instanceScope.Credentials.Spec.SSHPort),
	}

	var stdin io.Reader

	// If the sudo password is set, we need to pipe it to the ssh command.

	if instanceScope.Credentials.Spec.SudoPasswordEncoded != "" {
		pass, err := base64.StdEncoding.DecodeString(instanceScope.Credentials.Spec.SudoPasswordEncoded)
		if err != nil {
			return err
		}
		stdin = bytes.NewBuffer(pass)

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
		stdout = genssh.NewLogger(instanceScope.Logger.WithName("stdout"))
	}

	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if stderr == nil {
		stderr = genssh.NewLogger(instanceScope.Logger.WithName("stderr"))
	}

	instanceScope.Logger.Info(
		"Exec ssh command",
		"user", instanceScope.Credentials.Spec.User,
		"address", instanceScope.Instance.Spec.Address,
		"port", instanceScope.Credentials.Spec.SSHPort,
		"command", formatCommand(cmd.Args),
		"args", formatArgs(cmd.Args),
	)

	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "failed to run ssh command")
	}

	return nil
}

// ExecSSHCommandToString executes a command on the StaticInstance and returns the output as a string.
func (s *SSH) ExecSSHCommandToString(instanceScope *scope.InstanceScope, command string) (string, error) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	err := s.ExecSSHCommand(instanceScope, command, stdout, stderr)
	if err != nil {
		stderrBytes, err2 := io.ReadAll(stderr)
		if err2 != nil {
			return "", errors.Wrap(err2, "failed to read stderr from ssh command")
		}
		str := strings.TrimSpace(string(stderrBytes))
		logSSHOutput(instanceScope.Logger, "stderr", str)
		return str, err
	}

	stdoutBytes, err := io.ReadAll(stdout)
	if err != nil {
		return "", errors.Wrap(err, "failed to read stdout from ssh command")
	}

	return strings.TrimSpace(string(stdoutBytes)), nil
}

func formatCommand(args []string) string {
	const maxCommandLength = 1024
	runes := []rune(strings.Join(args, " "))
	if len(runes) <= maxCommandLength {
		return string(runes)
	}

	return fmt.Sprintf("%s... (truncated, total length %d)", string(runes[:maxCommandLength]), len(runes))
}

func formatArgs(args []string) []string {
	const maxArgLength = 512

	formatted := make([]string, 0, len(args))
	for _, arg := range args {
		runes := []rune(arg)
		if len(runes) <= maxArgLength {
			formatted = append(formatted, arg)
			continue
		}

		formatted = append(formatted, fmt.Sprintf("%s... (truncated, total length %d)", string(runes[:maxArgLength]), len(runes)))
	}

	return formatted
}

func logSSHOutput(logger logr.Logger, stream, raw string) {
	if raw == "" {
		return
	}

	text := strings.ReplaceAll(raw, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	for _, line := range strings.Split(text, "\n") {
		logger.Info(line, "stream", stream)
	}
}
