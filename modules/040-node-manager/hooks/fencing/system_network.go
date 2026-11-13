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

package fencing

import (
	"context"
	"fmt"
	"slices"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const (
	systemNetworkNotFoundMetric = "d8_node_manager_system_network_not_found"
	systemNetworkCRD            = "systemnetworks.network.deckhouse.io"

	moduleConfigSnapshot     = "module_config"
	systemNetworkCRDSnapshot = "system_network_crd"
)

var systemNetworkGVR = schema.GroupVersionResource{Group: "network.deckhouse.io", Version: "v1alpha1", Resource: "systemnetworks"}

func applySystemNetworkNameFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	// A value of another type cannot pass the schema of the module settings: it is not a name.
	name, _, _ := unstructured.NestedString(obj.Object, "spec", "settings", "fencing", "systemNetworkFallback", "systemNetworkName")
	return name, nil
}

// The hook only writes metrics, so it has a queue of its own and does not wait for the hooks that write values.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:    "/modules/node-manager/fencing-system-network",
	Schedule: []go_hook.ScheduleConfig{{Name: "system_network", Crontab: "*/30 * * * * *"}},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:         moduleConfigSnapshot,
			ApiVersion:   "deckhouse.io/v1alpha1",
			Kind:         "ModuleConfig",
			NameSelector: &types.NameSelector{MatchNames: []string{"node-manager"}},
			FilterFunc:   applySystemNetworkNameFilter,
		},
		// The CRD is only a trigger: the hook reads the network itself.
		{
			Name:         systemNetworkCRDSnapshot,
			ApiVersion:   "apiextensions.k8s.io/v1",
			Kind:         "CustomResourceDefinition",
			NameSelector: &types.NameSelector{MatchNames: []string{systemNetworkCRD}},
			FilterFunc:   nameFilter,
		},
	},
}, dependency.WithExternalDependencies(handleSystemNetwork))

// handleSystemNetwork publishes a series for the SystemNetwork that the ModuleConfig names and that does not
// exist. It changes neither the settings nor the ConfigMap of the agents.
func handleSystemNetwork(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	names, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, moduleConfigSnapshot)
	if err != nil {
		return fmt.Errorf("failed to unmarshal '%s' snapshot: %w", moduleConfigSnapshot, err)
	}

	// The setting exists only in these editions.
	edition := input.Values.Get("global.deckhouseEdition").String()
	if len(names) == 0 || names[0] == "" || !slices.Contains([]string{"EE", "FE", "CSE"}, edition) {
		input.MetricsCollector.Expire(systemNetworkNotFoundMetric)
		return nil
	}
	name := names[0]

	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("get k8s client: %w", err)
	}
	// A direct request: it needs no discovery, so it works while the CRD is absent.
	_, err = k8sClient.Dynamic().Resource(systemNetworkGVR).Get(ctx, name, metav1.GetOptions{})
	switch {
	case err == nil:
		input.MetricsCollector.Expire(systemNetworkNotFoundMetric)
	case apierrors.IsNotFound(err):
		// The API gives the same answer for a missing object, group and version.
		input.MetricsCollector.Expire(systemNetworkNotFoundMetric)
		input.MetricsCollector.Set(systemNetworkNotFoundMetric, 1, map[string]string{"network": name}, metrics.WithGroup(systemNetworkNotFoundMetric))
	default:
		// The metric keeps its value: the hook does not know the state of the network.
		return fmt.Errorf("get SystemNetwork %q: %w", name, err)
	}

	return nil
}
