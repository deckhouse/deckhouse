//go:build !linux

package controller

import (
	envmgr "github.com/flant/addon-operator/pkg/module_manager/environment_manager"
)

func getChrootObjectDescriptors() []envmgr.ObjectDescriptor {
	return []envmgr.ObjectDescriptor{
		{
			Source:            "/proc/sys/kernel/cap_last_cap",
			Type:              envmgr.File,
			TargetEnvironment: envmgr.EnabledScriptEnvironment,
		},
		{
			Source:            "/deckhouse/shell_lib.sh",
			Type:              envmgr.File,
			TargetEnvironment: envmgr.EnabledScriptEnvironment,
		},
		{
			Target:            "/dev/null",
			Type:              envmgr.DevNull,
			TargetEnvironment: envmgr.EnabledScriptEnvironment,
		},
	}
}
