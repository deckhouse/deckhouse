package app

import (
	"fmt"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	AppName = "deckhouse-candi"
)

var (
	AppVersion = "dev"

	ConfigPath  = ""
	SanityCheck = false
)

func DefineConfigFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("config", "Config file path").
		Required().
		StringVar(&ConfigPath)
}

func DefineSanityFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("yes-i-am-sane-and-i-understand-what-i-am-doing", "You should double check what you are doing here").
		BoolVar(&SanityCheck)
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
