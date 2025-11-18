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

package ssh_testing

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
)

type sshContainer struct {
	PublicKey     string
	PublicKeyPath string
	Password      string
	Username      string
	Port          int
	SudoAccess    bool
	id            string
	IP            string
	configPath    string
	network       string
}

func NewSSHContainer(publicKey, publicKeyPath, password, username string, port int, sudoAccess bool) *sshContainer {
	return &sshContainer{
		PublicKey:     publicKey,
		PublicKeyPath: publicKeyPath,
		Password:      password,
		Username:      username,
		Port:          port,
		SudoAccess:    sudoAccess,
	}
}

func (c *sshContainer) Cmd() (args []string) {
	args = []string{"run", "-d", "-e", "USER_NAME=" + c.Username, "-p", strconv.Itoa(c.Port) + ":2222"}
	if len(c.network) > 0 {
		args = append(args, "--network")
		args = append(args, c.network)
	}
	if len(c.PublicKey) > 0 {
		args = append(args, "-e")
		args = append(args, "PUBLIC_KEY="+c.PublicKey)
	}
	if len(c.PublicKeyPath) > 0 {
		args = append(args, "-e")
		args = append(args, "PUBLIC_KEY_FILE="+c.PublicKeyPath)
	}
	// set default password if no auth methods present
	if len(c.PublicKey) == 0 && len(c.PublicKeyPath) == 0 && len(c.Password) == 0 {
		c.Password = "password"
	}
	if len(c.Password) > 0 {
		args = append(args, "-e")
		args = append(args, "PASSWORD_ACCESS=true")
		args = append(args, "-e")
		args = append(args, "USER_PASSWORD="+c.Password)
	}
	args = append(args, "-e")
	args = append(args, "SUDO_ACCESS="+fmt.Sprintf("%v", c.SudoAccess))
	args = append(args, "--restart")
	args = append(args, "unless-stopped")

	if c.configPath != "" {
		args = append(args, "-v")
		args = append(args, c.configPath+":/config/sshd/sshd_config")
	}

	args = append(args, "lscr.io/linuxserver/openssh-server:10.0_p1-r9-ls209")

	return
}

func (c *sshContainer) String() string {
	cmd := "docker " + strings.Join(c.Cmd(), " ")
	return cmd
}

// force AllowTcpForwarding yes to allow connection throufh bastion
func (c *sshContainer) WriteConfig() error {
	tmpDir, err := os.MkdirTemp(".", "sshd")
	if err != nil {
		return err
	}
	conf, err := os.CreateTemp(tmpDir, "sshd_config")
	if err != nil {
		return err
	}
	config := `Port 2222
AuthorizedKeysFile	.ssh/authorized_keys
`
	if len(c.Password) > 0 {
		config = config + `PasswordAuthentication yes`
	} else {
		config = config + `PasswordAuthentication no`
	}
	config = config + `
AllowTcpForwarding yes
GatewayPorts no
X11Forwarding no
PidFile /config/sshd.pid
Subsystem	sftp	internal-sftp
`

	_, err = conf.WriteString(config)
	c.configPath = conf.Name()
	return err
}

func (c *sshContainer) RemoveConfig() error {
	path := filepath.Dir(c.configPath)
	return os.RemoveAll(path)
}

func (c *sshContainer) Start() error {
	args := c.Cmd()
	cmd := exec.Command("docker", args...)
	idBytes, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error: %v, output: %s", err, string(idBytes))
	}
	c.id = strings.TrimSpace(string(idBytes))
	time.Sleep(2 * time.Second)
	cmd = exec.Command("docker", "inspect", "-f", "'{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}'", c.id)
	ip, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error: %v, output: %s", err, string(ip))
	}
	c.IP = strings.TrimSpace(string(ip))
	c.IP = strings.ReplaceAll(c.IP, "'", "")

	return nil
}

func (c *sshContainer) Stop() error {
	if c.id != "" {
		cmd := exec.Command("docker", "stop", c.id)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("error: %v, output: %s", err, string(out))
		}
		cmd = exec.Command("docker", "rm", c.id)
		out, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("error: %v, output: %s", err, string(out))
		}
		c.id = ""
		if c.network != "" {
			cmd = exec.Command("docker", "network", "rm", c.network)
			out, err = cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("error: %v, output: %s", err, string(out))
			}
			c.network = ""
		}
	}
	return nil
}

func (c *sshContainer) GetId() string {
	return c.id
}

func (c *sshContainer) GetConfigPath() string {
	return c.configPath
}

func (c *sshContainer) CreateDeckhouseDirs() error {
	if c.id == "" {
		return fmt.Errorf("container seems to be not started. Call Start() first")
	}
	cmd := exec.Command("docker", "exec", c.id, "mkdir", "-p", app.DeckhouseNodeTmpPath)
	err := cmd.Run()
	if err != nil {
		return err
	}
	cmd = exec.Command("docker", "exec", c.id, "chmod", "-R", "777", app.DeckhouseNodeTmpPath)
	err = cmd.Run()

	return err
}

func (c *sshContainer) WithNetwork(name string) error {
	if c.id != "" {
		return fmt.Errorf("could not be used with running container")
	}

	if c.network != "" {
		// network already exists. just return
		return nil
	}

	if len(name) == 0 {
		name = "dhctl-test"
	}

	cmd := exec.Command("docker", "network", "create", name)
	err := cmd.Run()
	if err != nil {
		return err
	}
	c.network = name

	return nil
}

func (c *sshContainer) Disconnect() error {
	if c.id == "" {
		return fmt.Errorf("container seems to be not started. Call Start() first")
	}

	if c.network == "" {
		return fmt.Errorf("container seems to be not connected to named bridge. Call WithNetwork() first")
	}

	cmd := exec.Command("docker", "network", "disconnect", c.network, c.id)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error: %v, output: %s", err, string(out))
	}

	return nil
}

func (c *sshContainer) Connect() error {
	if c.id == "" {
		return fmt.Errorf("container seems to be not started. Call Start() first")
	}

	if c.network == "" {
		return fmt.Errorf("container seems to be not connected to named bridge. Call WithNetwork() first")
	}

	cmd := exec.Command("docker", "network", "connect", c.network, c.id)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error: %v, output: %s", err, string(out))
	}

	return nil
}
