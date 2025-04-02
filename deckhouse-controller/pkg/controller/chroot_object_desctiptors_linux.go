/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"syscall"

	envmgr "github.com/flant/addon-operator/pkg/module_manager/environment_manager"
)

func getChrootObjectDescriptors() []envmgr.ObjectDescriptor {
	return []envmgr.ObjectDescriptor{
		{
			Source:            "/deckhouse/python_lib",
			Flags:             syscall.MS_BIND | syscall.MS_RDONLY,
			Type:              envmgr.Mount,
			TargetEnvironment: envmgr.ShellHookEnvironment,
		},
		{
			Source:            "/deckhouse/candi",
			Flags:             syscall.MS_BIND | syscall.MS_RDONLY,
			Type:              envmgr.Mount,
			TargetEnvironment: envmgr.ShellHookEnvironment,
		},
		{
			Source:            "/deckhouse/helm_lib",
			Flags:             syscall.MS_BIND | syscall.MS_RDONLY,
			Type:              envmgr.Mount,
			TargetEnvironment: envmgr.ShellHookEnvironment,
		},
		{
			Source:            "/chroot/tmp",
			Target:            "/tmp",
			Flags:             syscall.MS_BIND,
			Type:              envmgr.Mount,
			TargetEnvironment: envmgr.EnabledScriptEnvironment,
		},
		{
			Source:            "/usr",
			Flags:             syscall.MS_BIND | syscall.MS_RDONLY,
			Type:              envmgr.Mount,
			TargetEnvironment: envmgr.EnabledScriptEnvironment,
		},
		{
			Source:            "/bin",
			Flags:             syscall.MS_BIND | syscall.MS_RDONLY,
			Type:              envmgr.Mount,
			TargetEnvironment: envmgr.EnabledScriptEnvironment,
		},
		{
			Source:            "/lib",
			Flags:             syscall.MS_BIND | syscall.MS_RDONLY,
			Type:              envmgr.Mount,
			TargetEnvironment: envmgr.EnabledScriptEnvironment,
		},
		{
			Source:            "/lib64",
			Flags:             syscall.MS_BIND | syscall.MS_RDONLY,
			Type:              envmgr.Mount,
			TargetEnvironment: envmgr.EnabledScriptEnvironment,
		},
		{
			Source:            "/deckhouse/shell_lib",
			Flags:             syscall.MS_BIND | syscall.MS_RDONLY,
			Type:              envmgr.Mount,
			TargetEnvironment: envmgr.EnabledScriptEnvironment,
		},
		{
			Source:            "/deckhouse/shell-operator",
			Flags:             syscall.MS_BIND | syscall.MS_RDONLY,
			Type:              envmgr.Mount,
			TargetEnvironment: envmgr.EnabledScriptEnvironment,
		},
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
