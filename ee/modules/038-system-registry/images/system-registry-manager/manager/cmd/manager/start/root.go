/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package start

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"system-registry-manager/cmd/manager/common"
	"system-registry-manager/internal"
	"system-registry-manager/pkg/cfg"
)

var (
	startCmd       = "start"
	startShortHelp = "..."
	startLongHelp  = `
	...
	`
)

func NewStartCmd() *cobra.Command {
	defaultFlagVars := common.DefaultFlagVars{}
	cmd := &cobra.Command{
		Use:   startCmd,
		Short: startShortHelp,
		Long:  startLongHelp,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			common.SetDefaultFlagsVars(&defaultFlagVars)
			if err := cfg.InitConfig(); err != nil {
				log.Fatalf("error initializing config: %v", err)
			}
			internal.StartManager()
			return nil
		},
	}
	common.AddDefaultFlags(cmd.Flags(), &defaultFlagVars)
	return cmd
}
