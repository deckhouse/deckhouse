/*
Copyright 2021 Flant JSC

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

package sandbox_runner

import (
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/otiai10/copy"
)

type sandboxConfig struct {
	cmd *exec.Cmd
}

type SandboxOption func(sandboxConfig) error

type EnvOption func(cmd *exec.Cmd, value string) *exec.Cmd

func Run(cmd *exec.Cmd, opts ...SandboxOption) *gexec.Session {
	sandboxConf := sandboxConfig{
		cmd: cmd,
	}

	for _, opt := range opts {
		err := opt(sandboxConf)
		Expect(err).ToNot(HaveOccurred())
	}

	session, err := gexec.Start(sandboxConf.cmd, nil, ginkgo.GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())

	session.Wait(time.Minute)
	return session
}

func WithFile(path string, contents []byte, envOpts ...EnvOption) SandboxOption {
	return func(conf sandboxConfig) error {
		err := os.WriteFile(path, contents, os.FileMode(0644))
		if err != nil {
			return err
		}

		for _, opt := range envOpts {
			opt(conf.cmd, path)
		}

		return nil
	}
}

func WithEnvSetToFilePath(envName string) EnvOption {
	return func(cmd *exec.Cmd, value string) *exec.Cmd {
		cmd.Env = append(cmd.Env, envName+"="+value)
		return cmd
	}
}

func WithSourceDirectory(fromPath string, toPath string) SandboxOption {
	return func(_ sandboxConfig) error {
		return copy.Copy(fromPath, toPath)
	}
}

func AsUser(uid, gid uint32) SandboxOption {
	return func(conf sandboxConfig) error {
		conf.cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uid, Gid: gid}}

		return nil
	}
}
