/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package start

import (
	// log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"system-registry-manager/cmd/manager/common"
	// "system-registry-manager/internal/config"
	// "system-registry-manager/internal/manager"
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
			Start()
			// if err := config.InitConfig(); err != nil {
			// 	log.Fatalf("Error initializing config: %v", err)
			// }
			// manager.StartManager()
			return nil
		},
	}
	common.AddDefaultFlags(cmd.Flags(), &defaultFlagVars)
	return cmd
}
