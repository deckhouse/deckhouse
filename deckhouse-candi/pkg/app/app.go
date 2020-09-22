package app

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	AppName = "deckhouse-candi"
)

var (
	TmpDirName = filepath.Join(os.TempDir(), "deckhouse-candi")
)

var (
	AppVersion = "dev"

	ConfigPath    = ""
	SanityCheck   = false
	DropCache     = false
	LoggerType    = "pretty"
	SkipResources = false
	IsDebug       = false
)

func init() {
	if os.Getenv("CANDI_DEBUG") == "yes" {
		IsDebug = true
	}
}

func GlobalFlags(cmd *kingpin.Application) {
	cmd.Flag("logger-type", "Format output of an deckhouse-candi in different ways.").
		Default("pretty").
		EnumVar(&LoggerType, "pretty", "simple")
}

func DefineSkipResourcesFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("skip-resources", "Do not wait resources deletion (pv, loadbalancers, machines) from the cluster.").
		Default("false").
		BoolVar(&SkipResources)
}

func DefineConfigFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("config", "Config file path").
		Required().
		StringVar(&ConfigPath)
}

func DefineSanityFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("yes-i-am-sane-and-i-understand-what-i-am-doing", "You should double check what you are doing here.").
		Default("false").
		BoolVar(&SanityCheck)
}

func DefineDropCacheFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("yes-i-want-to-drop-cache", "All cached information will be deleted from your local cache.").
		Default("false").
		BoolVar(&DropCache)
}

func Debugf(format string, a ...interface{}) {
	if IsDebug {
		fmt.Printf(format, a...)
	}
}
