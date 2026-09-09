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

	ycsettingsv2 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/hooks/internal/api/settings/v2"
)

// Placeholder settings that keep the module values satisfying their schema until
// hooks/yandex_cluster_configuration.go projects the legacy YandexClusterConfiguration onto
// them. The values match the ones openapi/conversions/v2.yaml writes when it converts
// ModuleConfig v1 settings, so an operator sees the same marker whichever path brought the
// cluster here — keep the two in sync.
const (
	placeholderCloudID         = "PLACEHOLDER_REPLACE_ME"
	placeholderFolderID        = "PLACEHOLDER_REPLACE_ME"
	placeholderLayout          = "Standard"
	placeholderNodeNetworkCIDR = "10.0.0.0/16"
	placeholderSSHPublicKey    = "ssh-rsa PLACEHOLDER_REPLACE_ME"
)

// The provider and nodes sections are required by openapi/config-values.yaml, and
// openapi/values.yaml pulls that schema in through x-extend. addon-operator merges the
// required lists of both schemas and checks the result on every values patch, not just
// before rendering the chart, so any hook that patches values while these sections are
// missing fails with "provider in body is required".
//
// On a cluster bootstrapped from a YandexClusterConfiguration alone there is no
// ModuleConfig to supply them, and the hook that fills them from the PCC runs at
// OnBeforeHelm order 20 — far too late. The TLS hooks registered by
// go_lib/hooks/tls_certificate patch values on their Kubernetes binding during
// Synchronization, and credentials.go does the same; a failed task is retried at the head
// of the main queue rather than dropped, so the module never reaches OnBeforeHelm at all.
//
// OnStartup runs in the Startup phase, ahead of Synchronization, which is what breaks that
// deadlock. With a ModuleConfig around this hook is a no-op: the sections are already in
// values, coming from config values.
//
// One gap remains by design: module-run skips onStartup hooks when a ModuleRun task arrives
// without DoModuleStartup, and a converge restart can displace the task that carries it.
// The bootstrap failure would come back until the next converge.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 6},
}, handleEnsureSettingsPlaceholders)

func handleEnsureSettingsPlaceholders(_ context.Context, input *go_hook.HookInput) error {
	// Both sections have to be set in a single hook run: addon-operator applies the whole
	// patch at once and validates the result, so a hook that seeded only one of them would
	// still fail on the other.
	if _, ok := input.Values.GetOk("cloudProviderYandex.provider"); !ok {
		input.Values.Set("cloudProviderYandex.provider", ycsettingsv2.Provider{
			Parameters: ycsettingsv2.ProviderParameters{
				CloudID:  placeholderCloudID,
				FolderID: placeholderFolderID,
			},
		})
	}

	if _, ok := input.Values.GetOk("cloudProviderYandex.nodes"); !ok {
		input.Values.Set("cloudProviderYandex.nodes", ycsettingsv2.Nodes{
			Parameters: ycsettingsv2.NodesParameters{
				Layout:          placeholderLayout,
				NodeNetworkCIDR: placeholderNodeNetworkCIDR,
				SSHPublicKey:    placeholderSSHPublicKey,
			},
		})
	}

	return nil
}
