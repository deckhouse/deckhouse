package app

import "gopkg.in/alecthomas/kingpin.v2"

var (
	RenderBashibleBundleDir = ""

	ParseInputFile = ""
	ParseOutput    = "json"
)

func DefineRenderConfigFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("bundle-dir", "Directory to render bashible bundle.").
		StringVar(&RenderBashibleBundleDir)
}

func DefineInputOutputRenderFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("file", "input file name with yaml documents").
		Short('f').
		StringVar(&ParseInputFile)

	cmd.Flag("output", "output format json or yaml").
		Short('o').
		EnumVar(&ParseOutput, "yaml", "json")
}
