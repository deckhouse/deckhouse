/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package common

import (
	"github.com/spf13/pflag"
	pkg_cfg "system-registry-manager/pkg/cfg"
)

type DefaultFlagVars struct {
	ConfigFilePath string
}

func AddDefaultFlags(f *pflag.FlagSet, flagVars *DefaultFlagVars) {
	f.StringVarP(&flagVars.ConfigFilePath, "config", "c", pkg_cfg.GetConfigFilePath(), "config.yaml filePath")
}

func SetDefaultFlagsVars(flagVars *DefaultFlagVars) {
	pkg_cfg.SetConfigFilePath(flagVars.ConfigFilePath)
}
