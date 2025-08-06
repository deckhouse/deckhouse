/*
Copyright 2025 Flant JSC

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

package gossh

import (
	"bytes"
	"caps-controller-manager/internal/scope"
	genssh "caps-controller-manager/internal/ssh"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

type SSH struct {
	sshClient *ssh.Client
}

func CreateSSHClient(instanceScope *scope.InstanceScope) (*SSH, error) {
	var signer ssh.Signer
	var err error
	var pass string
	if len(instanceScope.Credentials.Spec.SudoPasswordEncoded) > 0 {
		passBytes, err := base64.StdEncoding.DecodeString(instanceScope.Credentials.Spec.SudoPasswordEncoded)
		if err != nil {
			return nil, err
		}
		pass = string(passBytes)
	}
	AuthMethods := make([]ssh.AuthMethod, 0, 2)
	if len(instanceScope.Credentials.Spec.PrivateSSHKey) > 0 {
		privateSSHKey, err := base64.StdEncoding.DecodeString(instanceScope.Credentials.Spec.PrivateSSHKey)
		if err != nil {
			return nil, fmt.Errorf("privateSSHKey must be a valid base64 encoded string")
		}

		signer, err = ssh.ParsePrivateKey(privateSSHKey)
		if err != nil {
			return nil, fmt.Errorf("cannot parse keys")
		}
		AuthMethods = append(AuthMethods, ssh.PublicKeys(signer))
	}

	if len(pass) > 0 {
		AuthMethods = append(AuthMethods, ssh.Password(pass))
	}

	config := &ssh.ClientConfig{
		User:            instanceScope.Credentials.Spec.User,
		Auth:            AuthMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	addr := fmt.Sprintf("%s:%d", instanceScope.Instance.Spec.Address, instanceScope.Credentials.Spec.SSHPort)

	sshClient, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to SSH host %s", addr)
	}

	return &SSH{sshClient: sshClient}, nil
}

// ExecSSHCommand executes a command on the StaticInstance.
func (s *SSH) ExecSSHCommand(instanceScope *scope.InstanceScope, command string, stdout io.Writer, stderr io.Writer) error {
	var pass string
	if len(instanceScope.Credentials.Spec.SudoPasswordEncoded) > 0 {
		passBytes, err := base64.StdEncoding.DecodeString(instanceScope.Credentials.Spec.SudoPasswordEncoded)
		if err != nil {
			return err
		}
		pass = string(passBytes)
	}

	defer s.sshClient.Close()

	session, err := s.sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("cannot create session")
	}
	defer session.Close()

	if stdout == nil {
		stdout = genssh.NewLogger(instanceScope.Logger.WithName("stdout"))
	}

	if stderr == nil {
		stderr = genssh.NewLogger(instanceScope.Logger.WithName("stderr"))
	}

	session.Stdout = stdout
	session.Stderr = stderr

	command = fmt.Sprintf(`sudo -p SudoPassword -H -S -i bash -c "%s"`, command)
	// Set up a pipe to write to the session's stdin
	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	defer stdin.Close()

	if err := session.Start(command); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	stderrBuf := stderr.(*bytes.Buffer)
	stdoutBuf := stdout.(*bytes.Buffer)

	passwordSent := false
	for {
		if len(stderrBuf.Bytes()) > 0 {
			line := stderrBuf.String()
			if strings.Contains(line, "SudoPassword") {
				if !passwordSent {
					passwordSent = true
					if _, err := stdin.Write([]byte(pass + "\n")); err != nil {
						return fmt.Errorf("failed to write password to stdin: %w", err)
					}
				}

			}
		}
		if len(stdoutBuf.Bytes()) > 0 {
			break
		}
	}

	err = session.Wait()
	return err
}

// ExecSSHCommandToString executes a command on the StaticInstance and returns the output as a string.
func (s *SSH) ExecSSHCommandToString(instanceScope *scope.InstanceScope, command string) (string, error) {
	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}
	err := s.ExecSSHCommand(instanceScope, command, &stdout, &stderr)
	if err != nil {
		stderrBytes, err2 := io.ReadAll(&stderr)
		if err2 != nil {
			return "", errors.Wrap(err2, "failed to read stderr from ssh command")
		}
		str := strings.TrimSpace(string(stderrBytes))
		instanceScope.Logger.Info(str, "stderr")
		return str, err
	}

	return stdout.String(), nil
}
