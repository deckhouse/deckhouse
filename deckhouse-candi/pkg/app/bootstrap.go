package app

import "gopkg.in/alecthomas/kingpin.v2"

var (
	InternalNodeIP = ""
)

func DefineInternalNodeAddressFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("internal-node-ip", "Address of a node from internal network.").
		Required().
		StringVar(&InternalNodeIP)
}
