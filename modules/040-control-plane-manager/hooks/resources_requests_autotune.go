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
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

const (
	autotuneScheduleName = "autotune"
	autotuneQueue        = "/modules/control-plane-manager/autotune"

	autotuneStateCMName = "d8-control-plane-manager-resources-autotune-state"
	autotuneStateKey    = "state"

	autotuneMetricName  = "d8_control_plane_manager_resources_autotune_insufficient_capacity"
	autotuneMetricGroup = "D8ControlPlaneResourcesAutotuneInsufficientCapacity"

	snapshotCPMMC    = "CPMResourcesRequests"
	snapshotGlobalMC = "GlobalResourcesRequests"
	snapshotAutotune = "AutotuneState"
	snapshotNodes    = "NodesResources"
)

// Hook A (calculator): always writes per-component requests into the ConfigMap.
// Schedule evaluates PodMetrics; ModuleConfig/node sync applies MC split or legacy.
// Hook B (resources_requests_autotune_sync.go) only projects that CM into values.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: autotuneQueue,
	Schedule: []go_hook.ScheduleConfig{
		{Name: autotuneScheduleName, Crontab: "0 3 * * *"},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		controlPlaneNodesBinding(false, false),
		resourcesRequestsMCBinding(snapshotCPMMC, "control-plane-manager", applyCPMResourcesRequestsFilter, false, false),
		resourcesRequestsMCBinding(snapshotGlobalMC, "global", applyGlobalResourcesRequestsFilter, false, false),
		autotuneStateBinding(false, false),
	},
}, dependency.WithExternalDependencies(func(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	return runAutotune(ctx, input, dc, true)
}))

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: autotuneQueue,
	Kubernetes: []go_hook.KubernetesConfig{
		controlPlaneNodesBinding(true, true),
		resourcesRequestsMCBinding(snapshotCPMMC, "control-plane-manager", applyCPMResourcesRequestsFilter, true, true),
		resourcesRequestsMCBinding(snapshotGlobalMC, "global", applyGlobalResourcesRequestsFilter, true, true),
		// Read previous applied* for deadband/cooldown; events=false so our own
		// CreateOrUpdate does not re-enter Hook A.
		autotuneStateBinding(true, false),
	},
}, dependency.WithExternalDependencies(func(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	return runAutotune(ctx, input, dc, false)
}))

func resourcesRequestsMCBinding(
	name, mcName string,
	filter go_hook.FilterFunc,
	onSync, onEvents bool,
) go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:       name,
		ApiVersion: "deckhouse.io/v1alpha1",
		Kind:       "ModuleConfig",
		NameSelector: &types.NameSelector{
			MatchNames: []string{mcName},
		},
		FilterFunc:                   filter,
		ExecuteHookOnEvents:          ptr.To(onEvents),
		ExecuteHookOnSynchronization: ptr.To(onSync),
	}
}

// runAutotune calculates per-component requests and always persists them to the CM.
// evaluate=true (cron): may raise/lower from PodMetrics.
// evaluate=false (MC/node sync): MC split, keep previous, or legacy discovery split.
func runAutotune(ctx context.Context, input *go_hook.HookInput, dc dependency.Container, evaluate bool) error {
	nodes, err := sdkobjectpatch.UnmarshalToStruct[Node](input.Snapshots, snapshotNodes)
	if err != nil {
		return fmt.Errorf("unmarshal NodesResources snapshots: %w", err)
	}
	// Managed cloud — no master Nodes; leave CM/values alone.
	if len(nodes) == 0 {
		return nil
	}

	state, err := readAutotuneState(input)
	if err != nil {
		return err
	}
	if state == nil {
		state = make(autotuneState)
	}

	overrides, usedGlobal, err := readResourcesRequestsOverrides(input)
	if err != nil {
		return err
	}

	discoveryMilliCPU, discoveryMemory, ok := minMasterNodeBudget(nodes)
	if !ok {
		return nil
	}
	discoveryMilliCPU = discoveryMilliCPU * controlPlanePercent / 100
	discoveryMemory = discoveryMemory * controlPlanePercent / 100
	if discoveryMilliCPU <= 0 {
		return fmt.Errorf("cpu resources for allocating on master nodes must be greater than %dm", configEveryNodeMilliCPU)
	}
	if discoveryMemory <= 0 {
		return fmt.Errorf("memory resources for allocating on master nodes must be greater than %dMi", configEveryNodeMemory/1024/1024)
	}

	resolved, err := resolveCombinedBudgetFromOverrides(overrides, discoveryMilliCPU, discoveryMemory)
	if err != nil {
		return err
	}
	if usedGlobal {
		input.MetricsCollector.Expire(obsoleteGlobalResourcesRequestsMetricGroup)
		input.MetricsCollector.Set(
			obsoleteGlobalResourcesRequestsMetricName,
			1,
			map[string]string{},
			metrics.WithGroup(obsoleteGlobalResourcesRequestsMetricGroup),
		)
	} else {
		input.MetricsCollector.Expire(obsoleteGlobalResourcesRequestsMetricGroup)
	}

	otherByNode, err := fetchOtherRequestsByMasterNodes(ctx, dc, nodes)
	if err != nil {
		return fmt.Errorf("list non-control-plane pod requests on masters: %w", err)
	}
	fitCPU, fitMemBytes, _ := minMasterFitBudget(nodes, otherByNode)
	fit := map[resourceKind]int64{resourceCPU: fitCPU, resourceMemory: fitMemBytes}
	combined := map[resourceKind]int64{
		resourceCPU:    resolved.MilliCPU,
		resourceMemory: resolved.MemoryBytes,
	}
	overridden := map[resourceKind]bool{
		resourceCPU:    resolved.CPUFromConfig,
		resourceMemory: resolved.MemoryFromConfig,
	}
	now := dc.GetClock().Now().UTC()
	pmaActive := controlPlaneAutotuneActive(input)

	deficits := map[resourceKind]int64{}

	for _, kind := range []resourceKind{resourceCPU, resourceMemory} {
		if overridden[kind] {
			applyPercentSplit(state, kind, combined[kind], now)
			clearPendingRaise(state, kind)
			continue
		}

		if evaluate && pmaActive {
			recs, usageOK := fetchRecs(ctx, dc, fetchComponentUsage, kind, false, fit[kind], func(comp string, ferr error) {
				input.Logger.Warn("autotune: metrics API fetch failed", "resource", kind, "component", comp, "error", ferr)
			})
			if usageOK || len(recs) > 0 {
				if d := applyMetricsRecommendations(input, state, kind, recs, fit[kind], combined[kind], now); d > 0 {
					deficits[kind] = d
				}
			} else if !measurementHasAnyApplied(state[kind], kind) {
				applyPercentSplit(state, kind, combined[kind], now)
			}
			continue
		}

		// Sync / no PMA: keep previous applied*, else legacy %-split of discovery/override budget.
		if !measurementHasAnyApplied(state[kind], kind) {
			applyPercentSplit(state, kind, combined[kind], now)
		}
		if d := refreshPendingRaiseDeficit(state, kind, fit[kind]); d > 0 {
			deficits[kind] = d
		}
	}

	ensureFullComponents(state, combined, now)

	input.MetricsCollector.Expire(autotuneMetricGroup)
	for kind, deficit := range deficits {
		if deficit <= 0 {
			continue
		}
		input.MetricsCollector.Set(
			autotuneMetricName,
			float64(deficit),
			map[string]string{"resource": string(kind)},
			metrics.WithGroup(autotuneMetricGroup),
		)
	}

	return persistAutotuneState(input, state)
}

type resourcesRequestsOverride struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

func applyCPMResourcesRequestsFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	mc := &v1alpha1.ModuleConfig{}
	if err := sdk.FromUnstructured(obj, mc); err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}
	if mc.Spec.Settings == nil {
		return resourcesRequestsOverride{}, nil
	}
	settings := mc.Spec.Settings.GetMap()
	rr, _ := settings["resourcesRequests"].(map[string]any)
	return resourcesRequestsOverride{
		CPU:    quantityString(rr["cpu"]),
		Memory: quantityString(rr["memory"]),
	}, nil
}

func applyGlobalResourcesRequestsFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	mc := &v1alpha1.ModuleConfig{}
	if err := sdk.FromUnstructured(obj, mc); err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}
	if mc.Spec.Settings == nil {
		return resourcesRequestsOverride{}, nil
	}
	settings := mc.Spec.Settings.GetMap()
	modules, _ := settings["modules"].(map[string]any)
	rr, _ := modules["resourcesRequests"].(map[string]any)
	cp, _ := rr["controlPlane"].(map[string]any)
	return resourcesRequestsOverride{
		CPU:    quantityString(cp["cpu"]),
		Memory: quantityString(cp["memory"]),
	}, nil
}

func quantityString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case json.Number:
		return t.String()
	case float64:
		return fmt.Sprintf("%.0f", t)
	case int64:
		return fmt.Sprintf("%d", t)
	case int:
		return fmt.Sprintf("%d", t)
	default:
		return ""
	}
}

func readResourcesRequestsOverrides(input *go_hook.HookInput) (resourcesRequestsOverride, bool, error) {
	cpm, err := unmarshalOverrideSnapshot(input, snapshotCPMMC)
	if err != nil {
		return resourcesRequestsOverride{}, false, err
	}
	global, err := unmarshalOverrideSnapshot(input, snapshotGlobalMC)
	if err != nil {
		return resourcesRequestsOverride{}, false, err
	}

	out := resourcesRequestsOverride{}
	usedGlobal := false
	if cpm.CPU != "" {
		out.CPU = cpm.CPU
	} else if global.CPU != "" {
		out.CPU = global.CPU
		usedGlobal = true
	}
	if cpm.Memory != "" {
		out.Memory = cpm.Memory
	} else if global.Memory != "" {
		out.Memory = global.Memory
		usedGlobal = true
	}
	return out, usedGlobal, nil
}

func unmarshalOverrideSnapshot(input *go_hook.HookInput, name string) (resourcesRequestsOverride, error) {
	snaps := input.Snapshots.Get(name)
	if len(snaps) == 0 {
		return resourcesRequestsOverride{}, nil
	}
	var out resourcesRequestsOverride
	if err := snaps[0].UnmarshalTo(&out); err != nil {
		return out, fmt.Errorf("unmarshal %s snapshot: %w", name, err)
	}
	return out, nil
}

func resolveCombinedBudgetFromOverrides(
	overrides resourcesRequestsOverride,
	discoveryMilliCPU, discoveryMemory int64,
) (resolvedCombinedBudget, error) {
	out := resolvedCombinedBudget{
		MilliCPU:    discoveryMilliCPU,
		MemoryBytes: discoveryMemory,
	}
	if overrides.CPU != "" {
		q, err := resource.ParseQuantity(overrides.CPU)
		if err != nil {
			return out, fmt.Errorf("cannot parse cpu %q: %w", overrides.CPU, err)
		}
		out.MilliCPU = q.MilliValue()
		out.CPUFromConfig = true
	}
	if overrides.Memory != "" {
		q, err := resource.ParseQuantity(overrides.Memory)
		if err != nil {
			return out, fmt.Errorf("cannot parse memory %q: %w", overrides.Memory, err)
		}
		out.MemoryBytes = q.Value()
		out.MemoryFromConfig = true
	}
	return out, nil
}

type autotuneComponentState struct {
	AppliedMilliCPU *int64 `json:"appliedMilliCPU,omitempty"`
	AppliedBytes    *int64 `json:"appliedBytes,omitempty"`
	LastChange      string `json:"lastChange,omitempty"`
}

// PendingRaiseSum is the last raise total that failed the fit gate (no since/deficit
// in CM — deficit is emitted as a metric only).
type autotuneMeasurementState struct {
	Components      map[string]autotuneComponentState `json:"components,omitempty"`
	PendingRaiseSum int64                             `json:"pendingRaiseSum,omitempty"`
}

type autotuneState map[resourceKind]*autotuneMeasurementState

func applyAutotuneStateFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	cm := &v1.ConfigMap{}
	if err := sdk.FromUnstructured(obj, cm); err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}
	raw, ok := cm.Data[autotuneStateKey]
	if !ok || raw == "" {
		return make(autotuneState), nil
	}
	var st autotuneState
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		return nil, fmt.Errorf("unmarshal autotune state: %w", err)
	}
	if st == nil {
		st = make(autotuneState)
	}
	return st, nil
}

func readAutotuneState(input *go_hook.HookInput) (autotuneState, error) {
	snapshots := input.Snapshots.Get(snapshotAutotune)
	if len(snapshots) == 0 {
		return make(autotuneState), nil
	}
	var st autotuneState
	if err := snapshots[0].UnmarshalTo(&st); err != nil {
		return nil, fmt.Errorf("unmarshal AutotuneState snapshot: %w", err)
	}
	if st == nil {
		st = make(autotuneState)
	}
	return st, nil
}

func persistAutotuneState(input *go_hook.HookInput, state autotuneState) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal autotune state: %w", err)
	}

	cm := &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      autotuneStateCMName,
			Namespace: kubeSystemNS,
			Labels: map[string]string{
				"heritage": "deckhouse",
				"module":   "control-plane-manager",
			},
		},
		Data: map[string]string{
			autotuneStateKey: string(raw),
		},
	}

	input.PatchCollector.CreateOrUpdate(cm)
	return nil
}

func autotuneStateBinding(onSync, onEvents bool) go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:       snapshotAutotune,
		ApiVersion: "v1",
		Kind:       "ConfigMap",
		NamespaceSelector: &types.NamespaceSelector{
			NameSelector: &types.NameSelector{MatchNames: []string{kubeSystemNS}},
		},
		NameSelector: &types.NameSelector{
			MatchNames: []string{autotuneStateCMName},
		},
		FilterFunc:                   applyAutotuneStateFilter,
		ExecuteHookOnEvents:          ptr.To(onEvents),
		ExecuteHookOnSynchronization: ptr.To(onSync),
	}
}

// listPodsOnNode lists non-terminated pods scheduled on nodeName.
// Overridable in unit tests (client-go fakes do not index spec.nodeName).
var listPodsOnNode = listPodsOnNodeFromAPI

func listPodsOnNodeFromAPI(ctx context.Context, dc dependency.Container, nodeName string) ([]v1.Pod, error) {
	client, err := dc.GetK8sClient()
	if err != nil {
		return nil, fmt.Errorf("get k8s client: %w", err)
	}
	fieldSelector := fields.AndSelectors(
		fields.OneTermEqualSelector("spec.nodeName", nodeName),
		fields.OneTermNotEqualSelector("status.phase", string(v1.PodSucceeded)),
		fields.OneTermNotEqualSelector("status.phase", string(v1.PodFailed)),
	).String()
	list, err := client.CoreV1().Pods("").List(ctx, metav1.ListOptions{FieldSelector: fieldSelector})
	if err != nil {
		return nil, fmt.Errorf("list pods on node %s: %w", nodeName, err)
	}
	return list.Items, nil
}

func fetchOtherRequestsByMasterNodes(ctx context.Context, dc dependency.Container, nodes []Node) (map[string]nodeOtherRequests, error) {
	out := make(map[string]nodeOtherRequests, len(nodes))
	for i := range nodes {
		name := nodes[i].Name
		if name == "" {
			continue
		}
		items, err := listPodsOnNode(ctx, dc, name)
		if err != nil {
			return nil, err
		}
		var milliCPU, memoryBytes int64
		for j := range items {
			pod := &items[j]
			if isAutotunedControlPlanePod(pod) {
				continue
			}
			cpu, mem := otherRequestsFromPod(pod)
			milliCPU += cpu
			memoryBytes += mem
		}
		out[name] = nodeOtherRequests{MilliCPU: milliCPU, MemoryBytes: memoryBytes}
	}
	return out, nil
}

func measurementHasAnyApplied(m *autotuneMeasurementState, resourceName resourceKind) bool {
	if m == nil || m.Components == nil {
		return false
	}
	for _, comp := range controlPlaneComponents {
		if appliedValue(m.Components[comp], resourceName) > 0 {
			return true
		}
	}
	return false
}

func ensureMeasurement(state autotuneState, resourceName resourceKind) *autotuneMeasurementState {
	m := state[resourceName]
	if m == nil {
		m = &autotuneMeasurementState{Components: map[string]autotuneComponentState{}}
		state[resourceName] = m
	}
	if m.Components == nil {
		m.Components = map[string]autotuneComponentState{}
	}
	return m
}

func setApplied(cs *autotuneComponentState, resourceName resourceKind, val int64) {
	switch resourceName {
	case resourceCPU:
		cs.AppliedMilliCPU = ptr.To(val)
	case resourceMemory:
		cs.AppliedBytes = ptr.To(val)
	}
}

func clearPendingRaise(state autotuneState, resourceName resourceKind) {
	if m := state[resourceName]; m != nil {
		m.PendingRaiseSum = 0
	}
}

func applyPercentSplit(state autotuneState, resourceName resourceKind, combinedBudget int64, now time.Time) {
	if combinedBudget <= 0 {
		return
	}
	m := ensureMeasurement(state, resourceName)
	ts := now.Format(time.RFC3339)
	for _, comp := range controlPlaneComponents {
		cs := m.Components[comp]
		setApplied(&cs, resourceName, fallbackSplit(combinedBudget, componentFallbackPercent[comp]))
		cs.LastChange = ts
		m.Components[comp] = cs
	}
}

// ensureFullComponents fills any missing applied* slots from the %-split so the
// CM always carries a complete components map for Hook B.
func ensureFullComponents(state autotuneState, combined map[resourceKind]int64, now time.Time) {
	ts := now.Format(time.RFC3339)
	for _, kind := range []resourceKind{resourceCPU, resourceMemory} {
		m := ensureMeasurement(state, kind)
		for _, comp := range controlPlaneComponents {
			if appliedValue(m.Components[comp], kind) > 0 {
				continue
			}
			cs := m.Components[comp]
			setApplied(&cs, kind, fallbackSplit(combined[kind], componentFallbackPercent[comp]))
			if cs.LastChange == "" {
				cs.LastChange = ts
			}
			m.Components[comp] = cs
		}
	}
}

func refreshPendingRaiseDeficit(state autotuneState, resourceName resourceKind, fitBudget int64) int64 {
	m := state[resourceName]
	if m == nil || m.PendingRaiseSum <= 0 {
		return 0
	}
	if m.PendingRaiseSum <= fitBudget {
		m.PendingRaiseSum = 0
		return 0
	}
	return m.PendingRaiseSum - fitBudget
}

const (
	autotuneMinMilliCPU    = int64(10)
	autotuneMinMemoryBytes = int64(15 * 1000 * 1000) // 15M
)

var fetchComponentUsage = fetchComponentUsageFromMetricsAPI

func clampRecommendation(raw float64, resourceName resourceKind, nodeBudget int64) int64 {
	var v int64
	switch resourceName {
	case resourceCPU:
		v = int64(math.Ceil(raw * 1000))
		if v < autotuneMinMilliCPU {
			v = autotuneMinMilliCPU
		}
	case resourceMemory:
		v = int64(math.Round(raw))
		if v < autotuneMinMemoryBytes {
			v = autotuneMinMemoryBytes
		}
	}
	if nodeBudget > 0 && v > nodeBudget {
		v = nodeBudget
	}
	return v
}

func fetchRecs(
	ctx context.Context,
	dc dependency.Container,
	fetch func(context.Context, dependency.Container, string, resourceKind) (float64, bool, error),
	resourceName resourceKind,
	overridden bool,
	nodeBudget int64,
	onErr func(component string, err error),
) (map[string]int64, bool) {
	recs := make(map[string]int64, len(controlPlaneComponents))
	usageOK := true
	if overridden {
		return recs, true
	}
	for _, comp := range controlPlaneComponents {
		v, ok, ferr := fetch(ctx, dc, comp, resourceName)
		if ferr != nil {
			usageOK = false
			if onErr != nil {
				onErr(comp, ferr)
			}
			continue
		}
		if ok {
			recs[comp] = clampRecommendation(v, resourceName, nodeBudget)
		}
	}
	return recs, usageOK
}

type customMetricValueList struct {
	Items []struct {
		Value resource.Quantity `json:"value"`
	} `json:"items"`
}

func fetchComponentUsageFromMetricsAPI(ctx context.Context, dc dependency.Container, component string, resourceName resourceKind) (float64, bool, error) {
	client, err := dc.GetK8sClient()
	if err != nil {
		return 0, false, fmt.Errorf("get k8s client: %w", err)
	}

	container := componentContainer[component]
	metric := "d8-cpm-autotune-" + string(resourceName)

	pods, err := client.CoreV1().Pods(kubeSystemNS).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("component=%s,tier=control-plane", container),
	})
	if err != nil {
		return 0, false, fmt.Errorf("list control-plane pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return 0, false, fmt.Errorf("no pods matching component=%s,tier=control-plane", container)
	}

	var maxVal float64
	found := false
	var lastErr error
	for i := range pods.Items {
		v, ok, ferr := fetchPodMetric(ctx, client, pods.Items[i].Name, metric)
		if ferr != nil {
			lastErr = ferr
			continue
		}
		if !ok {
			continue
		}
		if !found || v > maxVal {
			maxVal = v
			found = true
		}
	}
	if !found && lastErr != nil {
		return 0, false, lastErr
	}
	return maxVal, found, nil
}

func fetchPodMetric(ctx context.Context, client k8s.Client, podName, metric string) (float64, bool, error) {
	path := fmt.Sprintf(
		"/apis/custom.metrics.k8s.io/v1beta1/namespaces/%s/pods/%s/%s",
		kubeSystemNS, podName, metric,
	)
	body, err := client.CoreV1().RESTClient().Get().AbsPath(path).Do(ctx).Raw()
	if err != nil {
		return 0, false, fmt.Errorf("GET %s: %w", path, err)
	}

	var list customMetricValueList
	if err := json.Unmarshal(body, &list); err != nil {
		return 0, false, fmt.Errorf("decode metrics response for %s: %w; body=%.256s", podName, err, body)
	}
	if len(list.Items) == 0 {
		return 0, false, fmt.Errorf("GET %s: empty MetricValueList", path)
	}

	v := list.Items[0].Value.AsApproximateFloat64()
	if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		return 0, false, fmt.Errorf("GET %s: non-positive metric value %q", path, list.Items[0].Value.String())
	}
	return v, true, nil
}

const (
	raiseCooldown = 24 * time.Hour
	lowerCooldown = 72 * time.Hour
)

type decideAction string

const (
	decideSkip  decideAction = "skip"
	decideRaise decideAction = "raise"
	decideLower decideAction = "lower"
)

func decide(rec, applied int64, lastChange, now time.Time) decideAction {
	if applied <= 0 {
		if rec > 0 {
			return decideRaise
		}
		return decideSkip
	}
	delta := float64(rec-applied) / float64(applied)
	switch {
	case delta > raiseThreshold:
		if now.Sub(lastChange) >= raiseCooldown || lastChange.IsZero() {
			return decideRaise
		}
	case delta < -lowerThreshold:
		if now.Sub(lastChange) >= lowerCooldown || lastChange.IsZero() {
			return decideLower
		}
	}
	return decideSkip
}

func appliedValue(cs autotuneComponentState, resourceName resourceKind) int64 {
	switch resourceName {
	case resourceCPU:
		if cs.AppliedMilliCPU != nil {
			return *cs.AppliedMilliCPU
		}
	case resourceMemory:
		if cs.AppliedBytes != nil {
			return *cs.AppliedBytes
		}
	}
	return 0
}

func effectiveApplied(cs autotuneComponentState, resourceName resourceKind, combined int64, comp string) int64 {
	if v := appliedValue(cs, resourceName); v > 0 {
		return v
	}
	return fallbackSplit(combined, componentFallbackPercent[comp])
}

// applyMetricsRecommendations applies raise/lower under deadband+cooldown.
// Returns deficit > 0 when a raise is blocked by the fit budget.
func applyMetricsRecommendations(
	input *go_hook.HookInput,
	state autotuneState,
	resourceName resourceKind,
	recs map[string]int64,
	nodeBudget int64,
	combinedBudget int64,
	now time.Time,
) int64 {
	if len(recs) == 0 {
		input.Logger.Warn("autotune: no usage datapoints from metrics API, leaving state unchanged", "resource", resourceName)
		return refreshPendingRaiseDeficit(state, resourceName, nodeBudget)
	}

	m := ensureMeasurement(state, resourceName)
	proposed := make(map[string]int64, len(controlPlaneComponents))
	actions := make(map[string]decideAction, len(controlPlaneComponents))
	anyRaise := false

	for _, comp := range controlPlaneComponents {
		eff := effectiveApplied(m.Components[comp], resourceName, combinedBudget, comp)
		proposed[comp] = eff

		rec, hasRec := recs[comp]
		if !hasRec {
			actions[comp] = decideSkip
			continue
		}

		var lastChange time.Time
		if s := m.Components[comp].LastChange; s != "" {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				lastChange = t
			}
		}
		action := decide(rec, eff, lastChange, now)
		actions[comp] = action
		if action == decideRaise || action == decideLower {
			proposed[comp] = rec
		}
		if action == decideRaise {
			anyRaise = true
		}
	}

	var deficit int64
	if anyRaise {
		var sum int64
		for _, comp := range controlPlaneComponents {
			sum += proposed[comp]
		}
		if sum > nodeBudget {
			deficit = sum - nodeBudget
			for _, comp := range controlPlaneComponents {
				if actions[comp] == decideRaise {
					proposed[comp] = effectiveApplied(m.Components[comp], resourceName, combinedBudget, comp)
					actions[comp] = decideSkip
				}
			}
			m.PendingRaiseSum = sum
			input.Logger.Info("autotune: raise blocked by capacity gate",
				"resource", resourceName, "deficit", deficit, "budget", nodeBudget, "proposedSum", sum)
		} else {
			m.PendingRaiseSum = 0
		}
	} else if m.PendingRaiseSum > 0 {
		deficit = refreshPendingRaiseDeficit(state, resourceName, nodeBudget)
	}

	ts := now.Format(time.RFC3339)
	for _, comp := range controlPlaneComponents {
		action := actions[comp]
		if action == decideSkip {
			continue
		}
		cs := m.Components[comp]
		setApplied(&cs, resourceName, proposed[comp])
		cs.LastChange = ts
		m.Components[comp] = cs
		input.Logger.Info("autotune: committed recommendation",
			"component", comp, "resource", resourceName, "action", action, "value", proposed[comp])
	}

	return deficit
}
