package app

import "gopkg.in/alecthomas/kingpin.v2"

var (
	RenderNodeIP            = ""
	RenderBashibleBundle    = ""
	RenderBashibleBundleDir = ""
)

func DefineRenderConfigFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("bundle-dir", "Directory to render bashible bundle.").
		StringVar(&RenderNodeIP)

	cmd.Flag("node-ip", "IP address of Kubernetes Master Node.").
		Required().
		StringVar(&RenderNodeIP)

	cmd.Flag("bundle", "Bashible bundle name").
		Required().
		StringVar(&RenderBashibleBundle)
}
