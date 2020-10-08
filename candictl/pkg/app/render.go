package app

import "gopkg.in/alecthomas/kingpin.v2"

var (
	RenderBashibleBundleDir = ""

	ParseInputFile = ""
	ParseOutput    = "json"

	Editor = ""
)

func DefineRenderConfigFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("bundle-dir", "Directory to render bashible bundle.").
		StringVar(&RenderBashibleBundleDir)
}

func DefineEditorConfigFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("editor", "Your favourite editor.").
		StringVar(&Editor)
}

func DefineInputOutputRenderFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("file", "Input file name with YAML-documents.").
		Short('f').
		StringVar(&ParseInputFile)

	cmd.Flag("output", "Output format (JSON or YAML).").
		Short('o').
		EnumVar(&ParseOutput, "yaml", "json")
}
