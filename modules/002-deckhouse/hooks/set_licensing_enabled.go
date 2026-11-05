/*
Copyright 2026 Flant JSC

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
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
)

// Whether licensing runs is settled before any hook and holds until a restart.
// The templates render licensing-crds.deckhouse.io only where it does: FE with
// licensing turned off leaves the licensing CRDs for the administrator to delete.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 1},
}, setLicensingEnabled)

func setLicensingEnabled(_ context.Context, input *go_hook.HookInput) error {
	input.Values.Set("deckhouse.internal.licensingEnabled", edition.LicensingEnabled())

	return nil
}
