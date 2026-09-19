/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/hooks/internal"
	zsettingsv2 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/hooks/internal/api/settings/v2"
)

// The provider and nodes sections are required by openapi/config-values.yaml, and
// openapi/values.yaml pulls that schema in through x-extend. addon-operator merges the required
// lists of both schemas and checks the result on every values patch, not just before rendering
// the chart, so any hook that patches values while these sections are missing fails with
// "provider in body is required".
//
// On a cluster bootstrapped from a ZvirtClusterConfiguration alone there is no ModuleConfig to
// supply them, and the hook that fills them from the PCC runs at OnBeforeHelm order 20 — far too
// late. credentials.go patches values on its Kubernetes binding during Synchronization, and a
// failed task is retried at the head of the main queue rather than dropped, so the module would
// never reach OnBeforeHelm at all.
//
// OnStartup runs in the Startup phase, ahead of Synchronization, which is what breaks that
// deadlock. With a ModuleConfig around this hook is a no-op: the sections are already in values,
// coming from config values.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 6},
}, handleEnsureSettingsPlaceholders)

func handleEnsureSettingsPlaceholders(_ context.Context, input *go_hook.HookInput) error {
	// Both sections have to be set in a single hook run: addon-operator applies the whole patch
	// at once and validates the result, so a hook that seeded only one of them would still fail
	// on the other.
	if _, ok := input.Values.GetOk("cloudProviderZvirt.provider"); !ok {
		input.Values.Set("cloudProviderZvirt.provider", zsettingsv2.Provider{
			Parameters: zsettingsv2.ProviderParameters{
				Server:    internal.PlaceholderServer,
				ClusterID: internal.PlaceholderClusterID,
			},
		})
	}

	if _, ok := input.Values.GetOk("cloudProviderZvirt.nodes"); !ok {
		input.Values.Set("cloudProviderZvirt.nodes", zsettingsv2.Nodes{
			Parameters: zsettingsv2.NodesParameters{
				SSHPublicKey: internal.PlaceholderSSHPublicKey,
				Layout:       internal.DefaultLayout,
			},
		})
	}

	return nil
}
