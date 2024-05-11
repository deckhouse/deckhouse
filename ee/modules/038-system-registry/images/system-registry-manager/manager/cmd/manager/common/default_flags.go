/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package common

import (
	"github.com/spf13/pflag"
	"system-registry-manager/internal/config"
)

type DefaultFlagVars struct {
	ConfigFilePath string
}

func AddDefaultFlags(f *pflag.FlagSet, flagVars *DefaultFlagVars) {
	f.StringVarP(&flagVars.ConfigFilePath, "config", "c", config.GetConfigFilePath(), "config.yaml filePath")
}

func SetDefaultFlagsVars(flagVars *DefaultFlagVars) {
	config.SetConfigFilePath(flagVars.ConfigFilePath)
}
