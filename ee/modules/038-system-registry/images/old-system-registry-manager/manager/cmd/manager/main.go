/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"system-registry-manager/internal"
	"system-registry-manager/pkg/cfg"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type DefaultFlagVars struct {
	ConfigFilePath string
}

func main() {
	flagVars := DefaultFlagVars{}
	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start the manager",
		Long:  "Start the system registry manager",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.SetConfigFilePath(flagVars.ConfigFilePath)
			if err := cfg.InitConfig(); err != nil {
				log.Fatalf("error initializing config: %v", err)
			}
			internal.StartManager()
			return nil
		},
	}

	startCmd.Flags().StringVarP(&flagVars.ConfigFilePath, "config", "c", cfg.GetConfigFilePath(), "config.yaml filePath")

	rootCmd := &cobra.Command{Use: "app"}
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.AddCommand(startCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
