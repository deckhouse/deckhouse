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

package resources

import (
	"context"
	"fmt"
	"strconv"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

// ConfigMap → values.internal.resourcesRequests.components. Computes nothing:
// resources_requests_autotune.go owns the ConfigMap.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        autotuneQueue,
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		autotuneStateBinding(true, true),
	},
}, dependency.WithExternalDependencies(syncResourcesRequestsFromConfigMap))

// Overridable in unit tests.
var getAutotuneStateCM = getAutotuneStateFromAPI

func syncResourcesRequestsFromConfigMap(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	input.MetricsCollector.Expire(autotuneSyncDegradedMetricGroup)
	input.MetricsCollector.Expire(autotuneIncompleteMetricGroup)

	raws, err := sdkobjectpatch.UnmarshalToStruct[autotuneStateRaw](input.Snapshots, snapshotAutotune)
	if err != nil {
		return fmt.Errorf("unmarshal AutotuneState snapshots: %w", err)
	}
	if len(raws) > 0 {
		state, perr := parseAutotuneState(raws[0].State)
		if perr != nil {
			// The autotune hook rewrites it from scratch on its next run.
			input.Logger.Error("autotune sync: unreadable state ConfigMap, keeping previous values", "error", perr)
			setAutotuneDegraded(input, autotuneSyncDegradedMetricGroup, degradedReasonBadState)
			return nil
		}
		return projectAutotuneStateToValues(input, state)
	}

	// The watch event reaches the informer cache asynchronously, so on the first
	// render the snapshot can lag behind the cluster.
	state, err := getAutotuneStateCM(ctx, dc)
	switch {
	case apierrors.IsNotFound(err):
		// Legal state: a managed control plane has no master Nodes, so the ConfigMap
		// is never written. Reporting it would light up every managed cluster.
		removeComponents(input)
		return nil
	case err != nil:
		// Values live in memory between ModuleRuns, so leaving them alone keeps the
		// last known good requests instead of dropping to the template fallback.
		input.Logger.Error("autotune sync: cannot read state ConfigMap, keeping previous values", "error", err)
		setAutotuneDegraded(input, autotuneSyncDegradedMetricGroup, degradedReasonReadThrough)
		return nil
	}

	input.Logger.Info("autotune sync: state read through the API, snapshot was empty")
	return projectAutotuneStateToValues(input, state)
}

func getAutotuneStateFromAPI(ctx context.Context, dc dependency.Container) (autotuneState, error) {
	client, err := dc.GetK8sClient()
	if err != nil {
		return nil, fmt.Errorf("get k8s client: %w", err)
	}
	cm, err := client.CoreV1().ConfigMaps(kubeSystemNS).Get(ctx, autotuneStateCMName, metav1.GetOptions{})
	if err != nil {
		// Unwrapped so the caller can tell NotFound from a transport error.
		return nil, err
	}
	return parseAutotuneState(cm.Data[autotuneStateKey])
}

func removeComponents(input *go_hook.HookInput) {
	input.Values.Remove(pathComponents)
}

func projectAutotuneStateToValues(input *go_hook.HookInput, state autotuneState) error {
	components := map[string]any{}
	for _, comp := range controlPlaneComponents {
		entry := map[string]any{}
		if m := state[resourceCPU]; m != nil {
			if cs, ok := m.Components[comp]; ok && cs.AppliedMilliCPU != nil {
				entry["milliCPU"] = *cs.AppliedMilliCPU
			}
		}
		if m := state[resourceMemory]; m != nil {
			if cs, ok := m.Components[comp]; ok && cs.AppliedBytes != nil {
				entry["memoryBytes"] = strconv.FormatInt(*cs.AppliedBytes, 10)
			}
		}
		if len(entry) > 0 {
			components[comp] = entry
		}
	}

	if len(components) == 0 {
		removeComponents(input)
		return nil
	}

	// A partial map is applied as-is: refusing it would leave all four components
	// on the template fallback instead of three correct ones and one default.
	for _, kind := range []resourceKind{resourceCPU, resourceMemory} {
		if measurementIsComplete(state[kind], kind) {
			continue
		}
		input.Logger.Warn("autotune sync: incomplete components map, applying what is available", "resource", kind)
		input.MetricsCollector.Set(
			autotuneIncompleteMetricName,
			1,
			map[string]string{"resource": string(kind)},
			metrics.WithGroup(autotuneIncompleteMetricGroup),
		)
	}

	input.Values.Set(pathComponents, components)
	return nil
}
