/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"

	"github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/hooks/internal"
	zicv1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/hooks/internal/api/instanceclass/v1"
	zpccv1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/hooks/internal/api/pcc/v1"
	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

// Drives the migration from the legacy ZvirtClusterConfiguration to the ModuleConfig v2 model.
//
// Three states are possible:
//
//   - the legacy configuration is gone and the new resources are in place: the migration
//     artifacts are removed and ModuleConfig is the only source of truth;
//   - the legacy configuration is present and the new resources are already applied: same as
//     above, except the legacy Secret still exists and keeps driving the infrastructure;
//   - the legacy configuration is present and the migration is incomplete: its fields are
//     projected onto the v2 settings so the module keeps rendering, and the bundle an admin has
//     to apply is (re)generated.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "provider_cluster_configuration",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{internal.PCCSecretNamespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{internal.PCCSecretName},
			},
			FilterFunc: internal.FilterPCCSecret,
		},
		{
			Name:       "module_config",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{internal.ModuleName},
			},
			FilterFunc: internal.FilterModuleConfig,
		},
		{
			Name:       "credential_secrets",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{internal.Namespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{cpapi.CredentialSecretName},
			},
			FilterFunc: internal.FilterCredentialSecret,
		},
		{
			Name:       "node_groups",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "NodeGroup",
			FilterFunc: internal.FilterNodeGroup,
		},
		{
			Name:       "candi_discovery_data",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{internal.Namespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{internal.CandiDiscoverySecretName},
			},
			FilterFunc: internal.FilterCandiDiscoverySecret,
		},
		{
			Name:       "zvirt_instance_classes",
			ApiVersion: zicv1.GroupVersionKind.GroupVersion().String(),
			Kind:       zicv1.ZvirtInstanceClassKind,
			FilterFunc: internal.FilterNamedResource,
		},
	},
}, handleZvirtClusterConfiguration)

func handleZvirtClusterConfiguration(_ context.Context, input *go_hook.HookInput) error {
	pccResult, pccPresent, err := unmarshalToOneStruct[internal.PCCSecretFilterResult](input.Snapshots, "provider_cluster_configuration")
	if err != nil {
		return fmt.Errorf("unmarshal provider_cluster_configuration snapshots: %w", err)
	}

	// The candi Secret first, then the legacy PCC payload, defaults on top — published in every
	// state, see internal.ResolveDiscoveryData.
	var pccForDiscovery *internal.PCCSecretFilterResult
	if pccPresent {
		pccForDiscovery = &pccResult
	}
	discoveryData, err := internal.ResolveDiscoveryData(input, pccForDiscovery)
	if err != nil {
		return err
	}
	input.Values.Set("cloudProviderZvirt.internal.providerDiscoveryData", discoveryData)

	if !pccPresent || pccResult.ProviderClusterConfig == nil {
		// The legacy configuration is gone. Once the credentials and the ModuleConfig v2 live in
		// the cluster, the new model is the only source of truth and the migration artifacts have
		// to go: while d8-module-is-migrating exists, ShouldSkipNewModelValidation keeps new-model
		// validation switched off.
		if internal.HasCredentialSecret(input) && internal.HasMigratedModuleConfig(input) {
			internal.DeleteMigrationArtifacts(input)
			return nil
		}

		// Neither the legacy configuration nor a migrated cluster: the module has nothing to
		// render from.
		return errors.New("kube-system/d8-provider-cluster-configuration secret not found and the cluster is not migrated")
	}

	pcc := *pccResult.ProviderClusterConfig

	// Templates still read internal.providerClusterConfiguration, so it is published in every
	// state where the legacy configuration exists — including after the bundle was applied. The
	// legacy Secret keeps driving the infrastructure until the templates are moved onto the v2
	// settings.
	input.Values.Set("cloudProviderZvirt.internal.providerClusterConfiguration", pcc)

	if internal.IsMigrationResourcesApplied(input, pcc) {
		internal.DeleteMigrationArtifacts(input)
		return nil
	}

	if err := validateProviderClusterConfig(pcc); err != nil {
		return fmt.Errorf("validate provider cluster config: %w", err)
	}

	if err := projectPCCToSettings(input, pcc); err != nil {
		return fmt.Errorf("project provider cluster configuration onto settings: %w", err)
	}

	return nil
}

// projectPCCToSettings writes the legacy fields into the v2 settings paths, so the module renders
// from the same values whether or not the admin has applied the bundle yet.
func projectPCCToSettings(input *go_hook.HookInput, pcc zpccv1.ZvirtProviderClusterConfiguration) error {
	settings := internal.BuildModuleConfigSettingsV2(pcc)

	// A ModuleConfig v2 the admin already wrote wins: it is the target of the migration, and
	// overwriting it with the projection would undo edits made after the bundle was applied.
	if !internal.HasMigratedModuleConfig(input) {
		input.Values.Set("cloudProviderZvirt.provider", settings.Provider)

		// The `disabled` flag belongs to the operator, through config values, and must survive
		// the projection; the section holds nothing else.
		nodes := settings.Nodes
		nodes.Disabled = input.Values.Get("cloudProviderZvirt.nodes.disabled").Bool()
		input.Values.Set("cloudProviderZvirt.nodes", nodes)
	}

	return setCredentialSecretsValuesIfAbsent(input, pcc)
}

// setCredentialSecretsValuesIfAbsent seeds the credentials from the legacy configuration while the
// managed Secret does not exist yet. Without it the workloads would render with an empty login and
// password for the whole duration of the migration.
func setCredentialSecretsValuesIfAbsent(input *go_hook.HookInput, pcc zpccv1.ZvirtProviderClusterConfiguration) error {
	existing := make(map[string]internal.CredentialSecretValues)
	if raw, ok := input.Values.GetOk("cloudProviderZvirt.internal.credentialSecrets"); ok {
		if err := json.Unmarshal([]byte(raw.Raw), &existing); err != nil {
			return fmt.Errorf("unmarshal credentialSecrets: %w", err)
		}
	}

	if _, ok := existing[cpapi.CredentialSecretName]; !ok && pcc.Provider.Username != "" {
		existing[cpapi.CredentialSecretName] = internal.CredentialSecretValues{
			AuthScheme: string(cpapi.AuthSchemeUserPassword),
			Identity:   pcc.Provider.Username,
			Secret:     pcc.Provider.Password,
		}
	}

	input.Values.Set("cloudProviderZvirt.internal.credentialSecrets", existing)

	return nil
}

// validateProviderClusterConfig ensures the legacy configuration carries the fields the projection
// cannot invent.
func validateProviderClusterConfig(pcc zpccv1.ZvirtProviderClusterConfiguration) error {
	if pcc.Provider.Server == "" {
		return errors.New("provider.server cannot be empty")
	}
	if pcc.Provider.Username == "" {
		return errors.New("provider.username cannot be empty")
	}
	if pcc.Provider.Password == "" {
		return errors.New("provider.password cannot be empty")
	}
	if pcc.ClusterID == "" {
		return errors.New("clusterID cannot be empty")
	}

	return nil
}

func unmarshalToOneStruct[T any](snapshots pkg.Snapshots, key string) (T, bool, error) {
	items, err := sdkobjectpatch.UnmarshalToStruct[T](snapshots, key)
	if err != nil {
		return *new(T), false, err
	}

	if len(items) == 0 {
		return *new(T), false, nil
	}

	return items[0], true, nil
}
