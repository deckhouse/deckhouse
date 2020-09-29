package app

import (
	"os"
	"path/filepath"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	TerraformStateDir = filepath.Join(os.TempDir(), "candictl")
)

func DefineTerraformFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("terraform-state-dir", "Directory to store terraform state.").
		StringVar(&TerraformStateDir)
}
