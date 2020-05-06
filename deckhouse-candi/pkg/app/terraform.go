package app

import "gopkg.in/alecthomas/kingpin.v2"

var TerraformStateDir = ""

func DefineTerraformFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("terraform-state-dir", "Directory to store terraform state.").
		StringVar(&TerraformStateDir)
}
