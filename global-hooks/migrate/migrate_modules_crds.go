/*
Copyright 2023 Flant JSC

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

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/hooks/ensure_crds"
)

/* Migration: Delete after Deckhouse release 1.53
This migration is implemented as a global hook because it must happen
before the rolling update of the validating webhook from the 002-deckhouse module.
Otherwise, the webhook will prevent any interactions with ExternalModule* resources.
*/

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(createModuleCRD))

func createModuleCRD(_ *go_hook.HookInput, dc dependency.Container) error {
	ensureRes := ensure_crds.EnsureCRDs("/deckhouse/modules/005-external-module-manager/crds/module-*.yaml", dc)
	return ensureRes.ErrorOrNil()
}
