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

// ConfigMap → values. Computes nothing: hook_autotune.go owns the ConfigMap.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        autotuneQueue,
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		autotuneStateBinding(true, true),
	},
}, dependency.WithExternalDependencies(runSync))

// Overridable in unit tests.
var readStateCM = readStateCMFromAPI

func runSync(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	input.MetricsCollector.Expire(autotuneSyncDegradedMetricGroup)
	input.MetricsCollector.Expire(autotuneIncompleteMetricGroup)

	raws, err := sdkobjectpatch.UnmarshalToStruct[autotuneStateRaw](input.Snapshots, snapshotAutotune)
	if err != nil {
		return fmt.Errorf("unmarshal AutotuneState snapshots: %w", err)
	}
	if len(raws) > 0 {
		state, err := parseAutotuneState(raws[0].State)
		if err != nil {
			input.Logger.Error("autotune sync: unreadable state ConfigMap, keeping previous values", "error", err)
			setAutotuneDegraded(input, autotuneSyncDegradedMetricGroup, degradedReasonBadState)
			return nil
		}
		return applyStateToValues(input, state)
	}

	// The informer cache fills asynchronously: on the first render the snapshot can
	// lag behind the cluster.
	state, err := readStateCM(ctx, dc)
	switch {
	case apierrors.IsNotFound(err):
		// A managed control plane never gets one written.
		removeComponents(input)
		return nil
	case err != nil:
		// Values live in memory between ModuleRuns, so leaving them alone keeps the
		// last known good requests instead of dropping to the template default.
		input.Logger.Error("autotune sync: cannot read state ConfigMap, keeping previous values", "error", err)
		setAutotuneDegraded(input, autotuneSyncDegradedMetricGroup, degradedReasonReadThrough)
		return nil
	}

	input.Logger.Info("autotune sync: state read through the API, snapshot was empty")
	return applyStateToValues(input, state)
}

func readStateCMFromAPI(ctx context.Context, dc dependency.Container) (autotuneState, error) {
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
	input.Values.Remove(componentsValuesPath)
}

func applyStateToValues(input *go_hook.HookInput, state autotuneState) error {
	components := map[string]any{}
	for _, comp := range controlPlaneComponents {
		entry := map[string]any{}
		if milliCPU := appliedRequest(state[resourceCPU], comp, resourceCPU); milliCPU > 0 {
			entry["milliCPU"] = milliCPU
		}
		if bytes := appliedRequest(state[resourceMemory], comp, resourceMemory); bytes > 0 {
			entry["memoryBytes"] = strconv.FormatInt(bytes, 10)
		}
		if len(entry) > 0 {
			components[comp] = entry
		}
	}

	if len(components) == 0 {
		removeComponents(input)
		return nil
	}

	// Refusing a partial map would put all four components on the template default.
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

	input.Values.Set(componentsValuesPath, components)
	return nil
}
