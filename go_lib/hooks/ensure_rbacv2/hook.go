/*
Copyright 2024 Flant JSC

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

package ensure_rbacv2

import (
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/hashicorp/go-multierror"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

func RegisterHook(moduleName, moduleScope string, pathsToCRDs []string) bool {
	return sdk.RegisterFunc(&go_hook.HookConfig{
		OnStartup: &go_hook.OrderedConfig{Order: 10},
	}, dependency.WithExternalDependencies(ensureHandler(moduleName, moduleScope, pathsToCRDs)))
}

func ensureHandler(moduleName, moduleScope string, pathsToCRDs []string) func(input *go_hook.HookInput, dc dependency.Container) error {
	return func(input *go_hook.HookInput, dc dependency.Container) error {
		result := ensure(moduleName, moduleScope, pathsToCRDs, dc)
		if result.ErrorOrNil() != nil {
			input.LogEntry.WithError(result).Error("ensure_rbacv2 failed")
		}
		return result.ErrorOrNil()
	}
}
func ensure(moduleName, moduleScope string, pathsToCRDs []string, dc dependency.Container) *multierror.Error {
	result := new(multierror.Error)

	client, err := dc.GetK8sClient()
	if err != nil {
		result = multierror.Append(result, err)
		return result
	}

	inst, err := newInstaller(moduleName, moduleScope, client, pathsToCRDs)
	if err != nil {
		result = multierror.Append(result, err)
		return result
	}

	return inst.Run(context.TODO())
}
