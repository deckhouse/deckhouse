package app

import (
	"fmt"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"

	sh_app "github.com/flant/shell-operator/pkg/app"
)

const (
	AppName = "deckhouse-candi"
)

var (
	AppVersion = "dev"

	ConfigPath = ""
)

func DefineKonvergeFlags(cmd *kingpin.CmdClause) {
	sh_app.DefineKubeClientFlags(cmd)
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
