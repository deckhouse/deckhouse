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

package validation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"github.com/deckhouse/deckhouse/go_lib/configtools"
	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/moduledependency"
)

// An empty/nil network map omits the group entirely.
func newControlPlaneManagerNetworkConfig(network map[string]string) *v1alpha1.ModuleConfig {
	cfg := newModuleConfigFull(controlPlaneManagerModuleName, boolPtr(true), "", "")
	cfg.Spec.Version = 1
	settings := map[string]any{}
	if len(network) > 0 {
		group := map[string]any{}
		for k, v := range network {
			group[k] = v
		}
		settings["network"] = group
	}
	cfg.Spec.Settings = v1alpha1.MakeMappedFields(settings)
	return cfg
}

func newClusterConfigurationNetworkSecret(podSubnetCIDR, serviceSubnetCIDR, podSubnetNodeCIDRPrefix string) *corev1.Secret {
	raw := "apiVersion: deckhouse.io/v1\nkind: ClusterConfiguration\n"
	if podSubnetCIDR != "" {
		raw += "podSubnetCIDR: \"" + podSubnetCIDR + "\"\n"
	}
	if serviceSubnetCIDR != "" {
		raw += "serviceSubnetCIDR: \"" + serviceSubnetCIDR + "\"\n"
	}
	if podSubnetNodeCIDRPrefix != "" {
		raw += "podSubnetNodeCIDRPrefix: \"" + podSubnetNodeCIDRPrefix + "\"\n"
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "d8-cluster-configuration", Namespace: "kube-system"},
		Data:       map[string][]byte{"cluster-configuration.yaml": []byte(raw)},
	}
}

// networkTestHandlerDeps mirrors buildHandler in validate_control_plane_manager_test.go: a storage
// with the module known (so the experimental/CR checks pass) and a manager that reports it enabled.
func networkTestHandlerDeps(t *testing.T) (*fakeModuleStorage, *fakeModuleManager) {
	t.Helper()
	const moduleName = controlPlaneManagerModuleName
	storage := &fakeModuleStorage{
		modules: map[string]*moduletypes.Module{moduleName: newStorageModule(t, moduleName, "", "")},
	}
	manager := &fakeModuleManager{enabled: map[string]bool{moduleName: true}}
	return storage, manager
}

func TestModuleConfigValidationHandler_ControlPlaneManagerNetwork(t *testing.T) {
	const moduleName = controlPlaneManagerModuleName

	// spec.version=1 with settings needs a non-nil conversions store; the default panics on Get.
	validator := configtools.NewValidator(nil, conversion.NewConversionsStore())

	t.Run("first write with no ClusterConfiguration is allowed", func(t *testing.T) {
		storage, manager := networkTestHandlerDeps(t)
		dependencyExtender := moduledependency.NewIExtenderMock(t)
		moduleCR := newModuleCR(moduleName, []string{"alpha"}, "")
		handler := newTestHandlerWithValidator(t, storage, manager, dependencyExtender, false, nil, validator, moduleCR)

		newCfg := newControlPlaneManagerNetworkConfig(map[string]string{"podSubnetCIDR": "10.111.0.0/16"})
		oldCfg := newControlPlaneManagerNetworkConfig(nil)
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		assert.True(t, resp.Allowed)
	})

	t.Run("first write matching ClusterConfiguration is allowed", func(t *testing.T) {
		storage, manager := networkTestHandlerDeps(t)
		dependencyExtender := moduledependency.NewIExtenderMock(t)
		moduleCR := newModuleCR(moduleName, []string{"alpha"}, "")
		handler := newTestHandlerWithValidator(t, storage, manager, dependencyExtender, false, nil, validator,
			moduleCR, newClusterConfigurationNetworkSecret("10.111.0.0/16", "", ""))

		newCfg := newControlPlaneManagerNetworkConfig(map[string]string{"podSubnetCIDR": "10.111.0.0/16"})
		oldCfg := newControlPlaneManagerNetworkConfig(nil)
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		assert.True(t, resp.Allowed)
	})

	t.Run("first write mismatching ClusterConfiguration is rejected", func(t *testing.T) {
		storage, manager := networkTestHandlerDeps(t)
		dependencyExtender := moduledependency.NewIExtenderMock(t)
		moduleCR := newModuleCR(moduleName, []string{"alpha"}, "")
		handler := newTestHandlerWithValidator(t, storage, manager, dependencyExtender, false, nil, validator,
			moduleCR, newClusterConfigurationNetworkSecret("10.99.0.0/16", "", ""))

		newCfg := newControlPlaneManagerNetworkConfig(map[string]string{"podSubnetCIDR": "10.111.0.0/16"})
		oldCfg := newControlPlaneManagerNetworkConfig(nil)
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		require.False(t, resp.Allowed)
		require.NotNil(t, resp.Result)
		assert.Contains(t, resp.Result.Message, "podSubnetCIDR")
		assert.Contains(t, resp.Result.Message, "10.99.0.0/16")
	})

	t.Run("changing an already-set value is forbidden", func(t *testing.T) {
		storage, manager := networkTestHandlerDeps(t)
		dependencyExtender := moduledependency.NewIExtenderMock(t)
		moduleCR := newModuleCR(moduleName, []string{"alpha"}, "")
		handler := newTestHandlerWithValidator(t, storage, manager, dependencyExtender, false, nil, validator, moduleCR)

		newCfg := newControlPlaneManagerNetworkConfig(map[string]string{"podSubnetCIDR": "10.222.0.0/16"})
		oldCfg := newControlPlaneManagerNetworkConfig(map[string]string{"podSubnetCIDR": "10.111.0.0/16"})
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		require.False(t, resp.Allowed)
		require.NotNil(t, resp.Result)
		assert.Contains(t, resp.Result.Message, "forbidden to change podSubnetCIDR")
	})

	t.Run("clearing an already-set value is forbidden", func(t *testing.T) {
		storage, manager := networkTestHandlerDeps(t)
		dependencyExtender := moduledependency.NewIExtenderMock(t)
		moduleCR := newModuleCR(moduleName, []string{"alpha"}, "")
		handler := newTestHandlerWithValidator(t, storage, manager, dependencyExtender, false, nil, validator, moduleCR)

		newCfg := newControlPlaneManagerNetworkConfig(nil)
		oldCfg := newControlPlaneManagerNetworkConfig(map[string]string{"podSubnetCIDR": "10.111.0.0/16"})
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		require.False(t, resp.Allowed)
		require.NotNil(t, resp.Result)
		assert.Contains(t, resp.Result.Message, "forbidden to change podSubnetCIDR")
	})

	t.Run("unchanged value is allowed even when it mismatches ClusterConfiguration", func(t *testing.T) {
		storage, manager := networkTestHandlerDeps(t)
		dependencyExtender := moduledependency.NewIExtenderMock(t)
		moduleCR := newModuleCR(moduleName, []string{"alpha"}, "")
		handler := newTestHandlerWithValidator(t, storage, manager, dependencyExtender, false, nil, validator,
			moduleCR, newClusterConfigurationNetworkSecret("10.99.0.0/16", "", ""))

		newCfg := newControlPlaneManagerNetworkConfig(map[string]string{"podSubnetCIDR": "10.111.0.0/16"})
		oldCfg := newControlPlaneManagerNetworkConfig(map[string]string{"podSubnetCIDR": "10.111.0.0/16"})
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		assert.True(t, resp.Allowed)
	})

	t.Run("allow-unsafe annotation bypasses the immutability guard", func(t *testing.T) {
		storage, manager := networkTestHandlerDeps(t)
		dependencyExtender := moduledependency.NewIExtenderMock(t)
		moduleCR := newModuleCR(moduleName, []string{"alpha"}, "")
		handler := newTestHandlerWithValidator(t, storage, manager, dependencyExtender, false, nil, validator, moduleCR)

		newCfg := newControlPlaneManagerNetworkConfig(map[string]string{"podSubnetCIDR": "10.222.0.0/16"})
		newCfg.Annotations = map[string]string{networkAllowUnsafeAnnotation: "true"}
		oldCfg := newControlPlaneManagerNetworkConfig(map[string]string{"podSubnetCIDR": "10.111.0.0/16"})
		review := newModuleConfigAdmissionReview("UPDATE", newCfg, oldCfg)

		resp := callHandler(t, handler, review)
		assert.True(t, resp.Allowed)
	})

	t.Run("DELETE with an already-set value falls back to the CC check and is rejected on mismatch", func(t *testing.T) {
		storage, manager := networkTestHandlerDeps(t)
		manager.enabled[moduleName] = false
		dependencyExtender := moduledependency.NewIExtenderMock(t)
		moduleCR := newModuleCR(moduleName, []string{"alpha"}, "")
		handler := newTestHandlerWithValidator(t, storage, manager, dependencyExtender, false, nil, validator,
			moduleCR, newClusterConfigurationNetworkSecret("10.99.0.0/16", "", ""))

		oldCfg := newControlPlaneManagerNetworkConfig(map[string]string{"podSubnetCIDR": "10.111.0.0/16"})
		oldCfg.Spec.Enabled = boolPtr(false)
		review := newModuleConfigAdmissionReview("DELETE", nil, oldCfg)

		resp := callHandler(t, handler, review)
		require.False(t, resp.Allowed)
		require.NotNil(t, resp.Result)
		assert.Contains(t, resp.Result.Message, "podSubnetCIDR")
	})
}

// TestModuleConfigOwnsNetworkField covers the predicate that lets the ClusterConfiguration webhook
// stand down for a field once ModuleConfig control-plane-manager owns it - the counterpart of
// TestModuleConfigOwnsKubernetesVersion for the network group.
func TestModuleConfigOwnsNetworkField(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	newClient := func(objs ...client.Object) client.Client {
		return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	}

	t.Run("no ModuleConfig — ClusterConfiguration still owns the field", func(t *testing.T) {
		assert.False(t, moduleConfigOwnsNetworkField(context.Background(), newClient(), "podSubnetCIDR"))
	})

	t.Run("ModuleConfig without the network group does not claim ownership", func(t *testing.T) {
		cfg := newControlPlaneManagerNetworkConfig(nil)
		assert.False(t, moduleConfigOwnsNetworkField(context.Background(), newClient(cfg), "podSubnetCIDR"))
	})

	t.Run("a set field is owned", func(t *testing.T) {
		cfg := newControlPlaneManagerNetworkConfig(map[string]string{"podSubnetCIDR": "10.111.0.0/16"})
		assert.True(t, moduleConfigOwnsNetworkField(context.Background(), newClient(cfg), "podSubnetCIDR"))
	})

	t.Run("ownership is per field", func(t *testing.T) {
		cfg := newControlPlaneManagerNetworkConfig(map[string]string{"podSubnetCIDR": "10.111.0.0/16"})
		assert.False(t, moduleConfigOwnsNetworkField(context.Background(), newClient(cfg), "serviceSubnetCIDR"))
	})

	t.Run("a different module's ModuleConfig is irrelevant", func(t *testing.T) {
		cfg := newControlPlaneManagerNetworkConfig(map[string]string{"podSubnetCIDR": "10.111.0.0/16"})
		cfg.SetName("node-manager")
		assert.False(t, moduleConfigOwnsNetworkField(context.Background(), newClient(cfg), "podSubnetCIDR"))
	})
}
