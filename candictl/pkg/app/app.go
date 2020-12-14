package app

import (
	"os"
	"path/filepath"

	"gopkg.in/alecthomas/kingpin.v2"
)

const AppName = "candictl"

var TmpDirName = filepath.Join(os.TempDir(), "candictl")

var (
	AppVersion = "dev"

	ConfigPath    = ""
	SanityCheck   = false
	LoggerType    = "pretty"
	SkipResources = false
	IsDebug       = false
)

func init() {
	if os.Getenv("CANDICTL_DEBUG") == "yes" {
		IsDebug = true
	}
}

func GlobalFlags(cmd *kingpin.Application) {
	cmd.Flag("logger-type", "Format logs output of a candictl in different ways.").
		Envar(configEnvName("LOGGER_TYPE")).
		Default("pretty").
		EnumVar(&LoggerType, "pretty", "simple", "json")
}

func DefineSkipResourcesFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("skip-resources", "Do not wait resources deletion (pv, loadbalancers, machines) from the cluster.").
		Default("false").
		Envar(configEnvName("SKIP_RESOURCES")).
		BoolVar(&SkipResources)
}

func DefineConfigFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("config", "Config file path").
		Required().
		Envar(configEnvName("CONFIG")).
		StringVar(&ConfigPath)
}

func DefineSanityFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("yes-i-am-sane-and-i-understand-what-i-am-doing", "You should double check what you are doing here.").
		Default("false").
		BoolVar(&SanityCheck)
}

func configEnvName(name string) string {
	return "CANDICTL_CLI_" + name
}
