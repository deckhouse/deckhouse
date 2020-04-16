package app

import (
	"fmt"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"

	sh_app "github.com/flant/shell-operator/pkg/app"
)

const (
	AppName = "deckhouse-cluster"
)

var (
	AppVersion = "dev"

	SshAgentPrivateKeys = "~/.ssh/id_rsa"
	SshBastionHost      = ""
	SshBastionUser      = os.Getenv("USER")
	SshUser             = os.Getenv("USER")
	SshHost             = ""
	SshExtraArgs        = ""

	ConfigPath = ""
)

func DefineKonvergeFlags(cmd *kingpin.CmdClause) {
	sh_app.DefineKubeClientFlags(cmd)
}

func DefineSshFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("ssh-agent-private-keys", "Paths to private keys. Those keys will be used to connect to servers and to the bastion. Can be specified multiple times (default: '~/.ssh/id_rsa')").
		StringVar(&SshAgentPrivateKeys)
	cmd.Flag("ssh-bastion-host", "Jumper (bastion) host to connect to servers (will be used both by terraform and ansible). Only IPs or hostnames are supported, name from ssh-config will not work.").
		StringVar(&SshBastionHost)
	cmd.Flag("ssh-bastion-user", "User to authenticate under when connecting to bastion (default: $USER)").
		StringVar(&SshBastionUser)
	cmd.Flag("ssh-user", "User to authenticate under (default: $USER)").
		StringVar(&SshUser)
	cmd.Flag("ssh-host", "SSH destination").
		StringVar(&SshHost)
	cmd.Flag("ssh-extra-args", "extra args for ssh commands (-vvv)").
		StringVar(&SshExtraArgs)
}

func DefineConfigFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("config", "Config file path").
		Required().
		StringVar(&ConfigPath)
}

var IsDebug = -1

func Debugf(format string, a ...interface{}) {
	if IsDebug == -1 {
		if os.Getenv("DEBUG") == "yes" {
			IsDebug = 1
		} else {
			IsDebug = 0
		}
	}
	if IsDebug == 1 {
		fmt.Printf(format, a...)
	}
}
