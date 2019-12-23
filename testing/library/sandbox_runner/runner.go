package sandbox_runner

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
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

	session.Wait(30 * time.Second)

	Expect(os.RemoveAll(cmd.Dir)).Should(Succeed())

	return session
}

func WithFile(path string, contents []byte, envOpts ...EnvOption) SandboxOption {
	return func(conf sandboxConfig) error {
		filePath := filepath.Join(path)

		err := ioutil.WriteFile(filePath, contents, os.FileMode(0644))
		if err != nil {
			return err
		}

		for _, opt := range envOpts {
			opt(conf.cmd, filePath)
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
	return func(conf sandboxConfig) error {
		return copy.Copy(fromPath, toPath)
	}
}
