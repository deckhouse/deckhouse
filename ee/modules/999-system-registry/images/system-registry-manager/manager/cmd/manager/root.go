package main

import (
	"fmt"
	"os"
	"system-registry-manager/cmd/manager/common"
	"system-registry-manager/cmd/manager/start"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	globalShortHelp = "..."
	globalLongHelp  = `
	...
	`
)

func newRootCmd(args []string) (*cobra.Command, error) {
	defaultFlagVars := common.DefaultFlagVars{}
	cmd := &cobra.Command{
		Short: globalShortHelp,
		Long:  globalLongHelp,
		Args:  common.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			common.SetDefaultFlagsVars(&defaultFlagVars)
			return fmt.Errorf("Unknown command")
		},
	}
	common.AddDefaultFlags(cmd.Flags(), &defaultFlagVars)
	cmd.AddCommand(start.NewStartCmd())
	return cmd, nil
}

func main() {
	cmd, err := newRootCmd(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
