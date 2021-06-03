package app

import (
	"time"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	ApplyInterval             = 30 * time.Minute
	AutoConvergeListenAddress = ":9101"
	RunningNodeName           = ""
)

func DefineAutoConvergeFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("converge-interval", "Period to converge terraform state").
		Envar(configEnvName("CONVERGE_INTERVAL")).
		DurationVar(&ApplyInterval)

	cmd.Flag("listen-address", "Address to expose metrics").
		Envar(configEnvName("LISTEN_ADDRESS")).
		StringVar(&AutoConvergeListenAddress)

	cmd.Flag("node-name", "Node name where running auto-converger pod").
		Envar(configEnvName("RUNNING_NODE_NAME")).
		StringVar(&RunningNodeName)
}
