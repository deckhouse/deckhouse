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
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	deckhousev1 "caps-controller-manager/api/deckhouse.io/v1alpha2"
	capsssh "caps-controller-manager/internal/ssh"
)

// sudoPromptPollInterval is how often the sudo password prompt is polled for.
const sudoPromptPollInterval = 100 * time.Millisecond

type SSH struct {
	sshClient *ssh.Client
	pass      string
}

// syncWriter forwards everything written to the underlying writer while keeping
// its own copy, so that the prompt polling loop can inspect the output without
// racing with the ssh session goroutines.
type syncWriter struct {
	mu  sync.Mutex
	out io.Writer
	buf bytes.Buffer
}

func (w *syncWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf.Write(p)

	return w.out.Write(p)
}

func (w *syncWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.buf.String()
}

func (w *syncWriter) Len() int {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.buf.Len()
}

func CreateSSHClient(host string, credentials deckhousev1.SSHCredentialsSpec) (*SSH, error) {
	var signer ssh.Signer
	var err error
	var pass string
	if len(credentials.SudoPasswordEncoded) > 0 {
		passBytes, err := base64.StdEncoding.DecodeString(credentials.SudoPasswordEncoded)
		if err != nil {
			return nil, err
		}
		pass = string(passBytes)
	}
	AuthMethods := make([]ssh.AuthMethod, 0, 2)
	if len(credentials.PrivateSSHKey) > 0 {
		privateSSHKey, err := base64.StdEncoding.DecodeString(credentials.PrivateSSHKey)
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
		User:            credentials.User,
		Auth:            AuthMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         capsssh.ConnectTimeout,
	}

	addr := fmt.Sprintf("%s:%d", host, credentials.SSHPort)

	sshClient, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to SSH host %s : %w", addr, err)
	}

	return &SSH{sshClient: sshClient, pass: pass}, nil
}

// ExecSSHCommand executes a command on the StaticInstance.
func (s *SSH) ExecSSHCommand(ctx context.Context, command string, stdout io.Writer, stderr io.Writer) error {
	if s.sshClient == nil {
		return fmt.Errorf("ssh client in nil")
	}

	defer s.sshClient.Close()

	ctx, cancel := context.WithTimeout(ctx, capsssh.CommandTimeout)
	defer cancel()

	session, err := s.sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("cannot create session: %w", err)
	}
	defer session.Close()

	if stdout == nil {
		stdout = &bytes.Buffer{}
	}

	if stderr == nil {
		stderr = &bytes.Buffer{}
	}

	stdoutWriter := &syncWriter{out: stdout}
	stderrWriter := &syncWriter{out: stderr}

	session.Stdout = stdoutWriter
	session.Stderr = stderrWriter

	command = fmt.Sprintf(`sudo -p SudoPassword -H -S -i bash -c 'echo SUDO-SUCCESS && %s'`, command)
	// Set up a pipe to write to the session's stdin
	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	defer stdin.Close()

	if err := session.Start(command); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// The session is closed by the deferred Close, which unblocks Wait below.
	go func() {
		<-ctx.Done()
		_ = session.Close()
	}()

	if err := s.answerSudoPrompt(ctx, stdin, stdoutWriter, stderrWriter); err != nil {
		return err
	}

	return session.Wait()
}

// answerSudoPrompt waits for the remote command to produce output, sending the sudo
// password once the prompt shows up on stderr.
func (s *SSH) answerSudoPrompt(ctx context.Context, stdin io.Writer, stdout, stderr *syncWriter) error {
	ticker := time.NewTicker(sudoPromptPollInterval)
	defer ticker.Stop()

	passwordSent := false

	for {
		if !passwordSent && strings.Contains(stderr.String(), "SudoPassword") {
			passwordSent = true
			if _, err := stdin.Write([]byte(s.pass + "\n")); err != nil {
				return fmt.Errorf("failed to write password to stdin: %w", err)
			}
		}

		if stdout.Len() > 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for the command output: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

// ExecSSHCommandToString executes a command on the StaticInstance and returns the output as a string.
func (s *SSH) ExecSSHCommandToString(ctx context.Context, command string) (string, error) {
	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}
	err := s.ExecSSHCommand(ctx, command, &stdout, &stderr)
	if err != nil {
		return stderr.String(), err
	}

	return stdout.String(), nil
}
