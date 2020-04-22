package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	SshAgentPrivateKeys = "~/.ssh/id_rsa"
	SshPrivateKeys      = make([]string, 0)
	SshBastionHost      = ""
	SshBastionUser      = os.Getenv("USER")
	SshUser             = os.Getenv("USER")
	SshHost             = ""
	SshPort             = ""
	SshExtraArgs        = ""
)

func DefineSshFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("ssh-agent-private-keys", "Paths to private keys. Those keys will be used to connect to servers and to the bastion. Can be specified multiple times (default: '~/.ssh/id_rsa')").
		StringVar(&SshAgentPrivateKeys)
	cmd.Flag("ssh-bastion-host", "Jumper (bastion) host to connect to servers (will be used both by terraform and ansible). Only IPs or hostnames are supported, name from ssh-config will not work.").
		StringVar(&SshBastionHost)
	cmd.Flag("ssh-bastion-user", "User to authenticate under when connecting to bastion (default: $USER)").
		StringVar(&SshBastionUser)
	cmd.Flag("ssh-user", "User to authenticate under (default: $USER)").
		StringVar(&SshUser)
	cmd.Flag("ssh-host", "SSH destination host").
		StringVar(&SshHost)
	cmd.Flag("ssh-port", "SSH destination port").
		StringVar(&SshPort)
	cmd.Flag("ssh-extra-args", "extra args for ssh commands (-vvv)").
		StringVar(&SshExtraArgs)

	cmd.PreAction(func(c *kingpin.ParseContext) (err error) {
		SshPrivateKeys, err = ParseSshPrivateKeyPaths(SshAgentPrivateKeys)
		if err != nil {
			return fmt.Errorf("ssh private keys: %v", err)
		}
		return nil
	})
}

func ParseSshPrivateKeyPaths(paths string) ([]string, error) {
	res := make([]string, 0)
	if paths == "" {
		return res, nil
	}
	keys := strings.Split(paths, ",")
	for _, k := range keys {
		if strings.HasPrefix(k, "~") {
			home := os.Getenv("HOME")
			if home == "" {
				return nil, fmt.Errorf("HOME is not defined for key '%s'", k)
			}
			k = strings.Replace(k, "~", home, 1)
		}

		keyPath, err := filepath.Abs(k)
		if err != nil {
			return nil, fmt.Errorf("get absolute path for '%s': %v", k, err)
		}
		res = append(res, keyPath)
	}
	return res, nil
}
