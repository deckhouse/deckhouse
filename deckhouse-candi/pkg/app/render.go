package app

import "gopkg.in/alecthomas/kingpin.v2"

var (
	RenderBashibleBundleDir = ""
)

func DefineRenderConfigFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("bundle-dir", "Directory to render bashible bundle.").
		StringVar(&RenderBashibleBundleDir)
}
