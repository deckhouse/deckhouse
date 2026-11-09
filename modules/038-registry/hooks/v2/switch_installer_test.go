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

package v2

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	dhctl_registry "github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	bashible_model "github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
	registry_requirements "github.com/deckhouse/deckhouse/modules/038-registry/requirements"
)

// TestTheGateAdmitsEveryNewInstallation builds `registry-bashible-config` the way the installer of
// this release does — the same configuration parsing, the same manifest builder, the same Secret — for
// every kind of installation, and hands it to the gate.
//
// Built with the installer's own functions rather than written out here, because the defect this
// guards against was exactly a copy that drifted: the gate was written against what the v1
// implementation leaves, and the installer's documents for an installation with no registry settings
// and for one from a bundle are, by their contents, a v1 `Direct` and a v1 `Local` cluster. Every one
// of those installations was refused, recorded as running the v1 implementation, and alerted on.
//
// Each case also states what the gate makes of the same document without the installer's mark, which
// is what a v1 cluster carrying that document looks like after a forced update. Those refusals are the
// gate working; the mark is what tells the two apart.
func TestTheGateAdmitsEveryNewInstallation(t *testing.T) {
	cases := []struct {
		name string
		docs []string

		// written is false where the installer writes no node configuration at all: the registry
		// module then manages nothing, and the gate has nothing of the installer's to read.
		written bool

		// unmarked is the gate's reason for the same document without the mark, empty where it admits
		// it anyway.
		unmarked string
	}{{
		// The CE default until dhctl/default-registry-ce-managed, and still the fallback where the
		// modules carry no registry schema: `Direct` with no agent.
		name:     "no registry settings at all",
		written:  true,
		unmarked: `the cluster is in the "Direct" mode of the v1 implementation`,
	}, {
		// The CE default from dhctl/default-registry-ce-managed on, which is this same ModuleConfig.
		name:    "a registry ModuleConfig with an upstream",
		docs:    []string{registryModuleConfig(ceUpstream)},
		written: true,
	}, {
		name:    "a registry ModuleConfig with an upstream and a cache",
		docs:    []string{registryModuleConfig(ceUpstream + "    storage:\n      cache: true\n")},
		written: true,
	}, {
		// `Local` on the node with the node services' endpoints, and no agent: the bootstrap's static
		// pod serves the images until the store this implementation renders takes them over.
		name:     "an installation from a bundle",
		docs:     []string{registryModuleConfig("storage:\n      cache: true\n")},
		written:  true,
		unmarked: `the cluster is in the "Local" mode of the v1 implementation`,
	}, {
		// Refused by dhctl/refuse-unmanaged-without-registry; until then the installer falls back to
		// the `Direct` default.
		name:     "a registry module that manages nothing, and no registry anywhere",
		docs:     []string{registryModuleConfigMode("Unmanaged")},
		written:  true,
		unmarked: `the cluster is in the "Direct" mode of the v1 implementation`,
	}, {
		name: "Unmanaged in the deckhouse ModuleConfig",
		docs: []string{deckhouseModuleConfig(
			"mode: Unmanaged\n      unmanaged:\n        imagesRepo: registry.example.com/deckhouse\n" +
				"        username: username\n        password: " + testPassword + "\n")},
		written: true,
	}, {
		// What Commander writes when nobody named a registry. Read as the registry's address, which
		// leaves the module managing nothing.
		name: "Direct in the deckhouse ModuleConfig",
		docs: []string{deckhouseModuleConfig(
			"mode: Direct\n      direct:\n        imagesRepo: registry.example.com/deckhouse\n")},
	}, {
		name: "Direct in the deckhouse ModuleConfig next to a registry ModuleConfig with an upstream",
		docs: []string{
			deckhouseModuleConfig("mode: Direct\n      direct:\n        imagesRepo: registry.example.com/deckhouse\n"),
			registryModuleConfig(ceUpstream),
		},
		written: true,
	}, {
		name: "the registry in InitConfiguration",
		docs: []string{"apiVersion: deckhouse.io/v1\nkind: InitConfiguration\ndeckhouse:\n" +
			"  imagesRepo: registry.example.com/deckhouse\n  registryScheme: HTTPS\n"},
	}}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			secret, written := installersNodeConfig(t, test.docs)
			require.Equal(t, test.written, written, "which installations write a node configuration")
			if !written {
				return
			}

			result, err := filterLegacyNodeConfig(secret)
			require.NoError(t, err)
			assert.Nil(t, result, "the installer's document was read as the v1 implementation's")

			// The installation as the gate sees it: nothing recorded by the v1 implementation, and a
			// node configuration that is the installer's, which the filter drops.
			requirements.RemoveValue(registry_requirements.ImplementationKey)
			input, collector := newSwitchInput(t, map[string]any{})
			require.NoError(t, handleSwitch(context.Background(), input))

			current := valuesAfter(t, input)
			assert.True(t, current.Enabled)
			assert.Empty(t, current.SwitchBlockedReason)
			recorded, _ := requirements.GetValue(registry_requirements.ImplementationKey)
			assert.Equal(t, ImplementationV2, recorded,
				"a new installation was recorded as running the v1 implementation")
			for _, op := range collector.CollectedMetrics() {
				assert.NotEqual(t, blockedMetric, op.Name, "a new installation raised D8RegistryMigrationPending")
			}

			// The same document without the mark: what a v1 cluster carrying it looks like.
			delete(secret.Object["metadata"].(map[string]any)["annotations"].(map[string]any),
				bashible_model.InstallerMarkAnnotation)
			unmarked, err := filterLegacyNodeConfig(secret)
			require.NoError(t, err)
			if unmarked == nil {
				assert.Empty(t, test.unmarked, "the document names the agent, so it never needed the mark")
				return
			}
			config := unmarked.(legacyNodeConfig)
			enabled, reason := gate{NodeConfig: &config}.decide()
			if test.unmarked == "" {
				assert.True(t, enabled, reason)
				return
			}
			assert.False(t, enabled)
			assert.Contains(t, reason, test.unmarked)
		})
	}
}

// installersNodeConfig is `registry-bashible-config` as the installer writes it for an installation
// configured by docs, on a cluster whose control plane it creates with containerd: the path of
// `dhctl bootstrap` from the parsed resources to the Secret it creates.
func installersNodeConfig(t *testing.T, docs []string) (*unstructured.Unstructured, bool) {
	t.Helper()

	provider, err := config.RegistryConfigProvider(docs)
	require.NoError(t, err)
	registryConfig, err := provider.Config(registry_const.CRIContainerdV1, true)
	require.NoError(t, err)

	written, data, err := registryConfig.Manifest().RegistryBashibleConfigSecretData(dhctl_registry.GeneratePKI)
	require.NoError(t, err)
	if !written {
		return nil, false
	}

	secret := manifests.RegistryBashibleConfigSecret(data)
	secret.APIVersion, secret.Kind, secret.Namespace = "v1", "Secret", "d8-system"
	object, err := runtime.DefaultUnstructuredConverter.ToUnstructured(secret)
	require.NoError(t, err)
	return &unstructured.Unstructured{Object: object}, true
}

// ceUpstream is the settings of a registry ModuleConfig naming the public CE registry, which is also
// what dhctl/default-registry-ce-managed writes when no registry is named anywhere.
const ceUpstream = "primary:\n      upstream:\n        host: registry.deckhouse.io\n        path: /deckhouse/ce\n"

func registryModuleConfig(settings string) string {
	return "apiVersion: deckhouse.io/v1alpha1\nkind: ModuleConfig\nmetadata:\n  name: registry\nspec:\n" +
		"  enabled: true\n  version: 1\n  settings:\n    mode: Managed\n    " + settings
}

func registryModuleConfigMode(mode string) string {
	return "apiVersion: deckhouse.io/v1alpha1\nkind: ModuleConfig\nmetadata:\n  name: registry\nspec:\n" +
		"  enabled: true\n  version: 1\n  settings:\n    mode: " + mode + "\n"
}

func deckhouseModuleConfig(registry string) string {
	return "apiVersion: deckhouse.io/v1alpha1\nkind: ModuleConfig\nmetadata:\n  name: deckhouse\nspec:\n" +
		"  enabled: true\n  version: 1\n  settings:\n    registry:\n      " + registry
}
