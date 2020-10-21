package app

import "gopkg.in/alecthomas/kingpin.v2"

var (
	InternalNodeIP = ""
	DevicePath     = ""

	ResourcesPath = ""
)

func DefineBashibleBundleFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("internal-node-ip", "Address of a node from internal network.").
		Required().
		Envar(configEnvName("INTERNAL_NODE_IP")).
		StringVar(&InternalNodeIP)
	cmd.Flag("device-path", "Path of kubernetes-data device.").
		Required().
		Envar(configEnvName("DEVICE_PATH")).
		StringVar(&DevicePath)
}

func DefineResourcesFlags(cmd *kingpin.CmdClause, isRequired bool) {
	cmd.Flag("resources", "Path to a file with declared Kubernetes resources in YAML format.").
		Envar(configEnvName("RESOURCES")).
		StringVar(&ResourcesPath)

	if isRequired {
		cmd.GetFlag("resources").Required()
	}
}
