package app

import "gopkg.in/alecthomas/kingpin.v2"

var (
	InternalNodeIP = ""

	ResourcesPath = ""
)

func DefineInternalNodeAddressFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("internal-node-ip", "Address of a node from internal network.").
		Required().
		StringVar(&InternalNodeIP)
}

func DefineResourcesFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("resources", "Path to a file with declared Kubernetes resources in YAML format.").
		StringVar(&ResourcesPath)
}
