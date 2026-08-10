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
	"maps"
	"math"
	"strconv"
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

// resources_requests_autotune.go: schedule → calculate per-component requests → ConfigMap.
// resources_requests_autotune_sync.go: projects that CM into values.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: autotuneQueue,
	Schedule: []go_hook.ScheduleConfig{
		{Name: autotuneScheduleName, Crontab: "0 */5 * * *"},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		controlPlaneNodesBinding(),
		// Events on both: otherwise the two documented ways to set the same budget
		// would behave differently — global would wait for the cron, the module MC
		// would apply at once.
		resourcesRequestsMCBinding(snapshotCPMMC, "control-plane-manager", applyCPMResourcesRequestsFilter),
		resourcesRequestsMCBinding(snapshotGlobalMC, "global", applyGlobalResourcesRequestsFilter),
		// events=false is mandatory: this hook writes that very ConfigMap.
		autotuneStateBinding(true, false),
	},
}, dependency.WithExternalDependencies(runAutotune))

func resourcesRequestsMCBinding(name, mcName string, filter go_hook.FilterFunc) go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:       name,
		ApiVersion: "deckhouse.io/v1alpha1",
		Kind:       "ModuleConfig",
		NameSelector: &types.NameSelector{
			MatchNames: []string{mcName},
		},
		FilterFunc:                   filter,
		ExecuteHookOnEvents:          ptr.To(true),
		ExecuteHookOnSynchronization: ptr.To(true),
	}
}

// runAutotune calculates per-component requests and always persists them to the CM.
//
// The only error it may return is a failure to marshal its own state: any other
// error would make addon-operator drop the whole PatchCollector, and the
// control-plane would fall back onto the 512m/512Mi template default.
func runAutotune(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	input.MetricsCollector.Expire(autotuneDegradedMetricGroup)
	input.MetricsCollector.Expire(autotuneMetricGroup)

	nodes, err := sdkobjectpatch.UnmarshalToStruct[Node](input.Snapshots, snapshotNodes)
	if err != nil {
		input.Logger.Error("autotune: cannot read master nodes snapshot", "error", err)
		setAutotuneDegraded(input, autotuneDegradedMetricGroup, degradedReasonBadNodes)
		return nil
	}
	// Managed control plane — the one legal case in which the ConfigMap does not
	// exist; the sync hook treats NotFound as normal for exactly this reason.
	if len(nodes) == 0 {
		return nil
	}

	state := readStateTolerant(input)
	overrides, usedGlobal := readOverridesTolerant(input)

	discovery := discoveryBudget(input, nodes)

	input.MetricsCollector.Expire(obsoleteGlobalResourcesRequestsMetricGroup)
	if usedGlobal {
		input.MetricsCollector.Set(
			obsoleteGlobalResourcesRequestsMetricName,
			1,
			map[string]string{},
			metrics.WithGroup(obsoleteGlobalResourcesRequestsMetricGroup),
		)
	}

	fit := map[resourceKind]int64{}
	fitOK := true
	otherByNode, err := fetchOtherRequestsByMasterNodes(ctx, dc, nodes)
	if err != nil {
		fitOK = false
		input.Logger.Error("autotune: cannot list non-control-plane pod requests on masters, skipping metrics path", "error", err)
		setAutotuneDegraded(input, autotuneDegradedMetricGroup, degradedReasonListPods)
	} else {
		fitCPU, fitMemBytes, _ := minMasterFitBudget(nodes, otherByNode)
		fit[resourceCPU] = fitCPU
		fit[resourceMemory] = fitMemBytes
	}

	now := dc.GetClock().Now().UTC()
	pmaActive := controlPlaneAutotuneActive(input)

	// What each measurement is split from: the override when one is usable.
	combined := maps.Clone(discovery)
	deficits := map[resourceKind]int64{}

	for _, kind := range []resourceKind{resourceCPU, resourceMemory} {
		ovr := overrides[kind]
		if !ovr.known {
			// Falling back to discovery would silently override the user's intent.
			continue
		}
		if !ovr.set && discovery[kind] <= 0 {
			// Nothing legitimate to compute from, see discoveryBudget.
			continue
		}
		m := ensureMeasurement(state, kind)

		if ovr.set {
			combined[kind] = ovr.value
			if m.AppliedOverride != nil && *m.AppliedOverride == ovr.value {
				continue
			}
			applyPercentSplit(state, kind, ovr.value, now)
			clearPendingRaise(state, kind)
			m.AppliedOverride = ptr.To(ovr.value)
			continue
		}

		if m.AppliedOverride != nil {
			// Override just removed: rebase now, not on the next nightly run.
			applyPercentSplit(state, kind, discovery[kind], now)
			m.AppliedOverride = nil
		}

		if !pmaActive {
			// Replaces any previous metric-based applied*.
			applyPercentSplit(state, kind, discovery[kind], now)
			clearPendingRaise(state, kind)
			continue
		}

		if !fitOK || !metricsRunDue(m, now) {
			continue
		}

		recs, usageOK := fetchRecs(ctx, input, dc, fetchComponentUsage, kind, fit[kind])
		// Recorded on the fact of the fetch, not on committing anything.
		m.LastMetricsRun = now.Format(time.RFC3339)
		if usageOK || len(recs) > 0 {
			if d := applyMetricsRecommendations(input, state, kind, recs, fit[kind], discovery[kind], now); d > 0 {
				deficits[kind] = d
			}
		} else if countApplied(state[kind], kind) == 0 {
			// Cold start before the series exist.
			applyPercentSplit(state, kind, discovery[kind], now)
		}
	}

	ensureFullComponents(state, combined, now)

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

// Rewriting the ConfigMap from scratch beats leaving the control-plane on the
// template fallback.
func readStateTolerant(input *go_hook.HookInput) autotuneState {
	state, err := readAutotuneState(input)
	if err != nil || state == nil {
		if err != nil {
			input.Logger.Warn("autotune: unreadable state ConfigMap, recomputing from scratch", "error", err)
			setAutotuneDegraded(input, autotuneDegradedMetricGroup, degradedReasonBadState)
		}
		return make(autotuneState)
	}
	return state
}

// discoveryBudget is the legacy per-measurement budget: the weakest master's
// effective resources, minus the every-node reservation the rest of the node
// needs anyway, carved down to controlPlanePercent.
//
// A master too small to give that up yields no budget, and the measurement is
// then left alone rather than reset to an invented number: with no ConfigMap
// entry the templates apply their own fallback.
func discoveryBudget(input *go_hook.HookInput, nodes []Node) map[resourceKind]int64 {
	out := map[resourceKind]int64{}
	effCPU, effMemory, ok := minMasterNodeBudget(nodes)
	if !ok {
		return out
	}
	out[resourceCPU] = (effCPU - configEveryNodeMilliCPU) * controlPlanePercent / 100
	out[resourceMemory] = (effMemory - configEveryNodeMemory) * controlPlanePercent / 100

	if out[resourceCPU] <= 0 || out[resourceMemory] <= 0 {
		input.Logger.Warn("autotune: master nodes leave nothing after the every-node reservation",
			"effectiveMilliCPU", effCPU, "effectiveMemoryBytes", effMemory,
			"reservedMilliCPU", configEveryNodeMilliCPU, "reservedMemoryBytes", configEveryNodeMemory)
		setAutotuneDegraded(input, autotuneDegradedMetricGroup, degradedReasonNodesTooSmall)
	}
	return out
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
		// cpu may be a bare number. %.0f would turn 0.5 into "0m" and 1.5 into "2000m".
		return strconv.FormatFloat(t, 'f', -1, 64)
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

// known=false means an override is configured but unusable.
type measurementOverride struct {
	known bool
	set   bool
	value int64 // millicpu for cpu, bytes for memory
}

// An unreadable snapshot marks both measurements unknown; an unparsable quantity
// only its own.
func readOverridesTolerant(input *go_hook.HookInput) (map[resourceKind]measurementOverride, bool) {
	out := map[resourceKind]measurementOverride{
		resourceCPU:    {known: true},
		resourceMemory: {known: true},
	}

	overrides, usedGlobal, err := readResourcesRequestsOverrides(input)
	if err != nil {
		input.Logger.Error("autotune: cannot read resourcesRequests overrides, leaving both measurements untouched", "error", err)
		setAutotuneDegraded(input, autotuneDegradedMetricGroup, degradedReasonBadOverride)
		return map[resourceKind]measurementOverride{
			resourceCPU:    {},
			resourceMemory: {},
		}, false
	}

	raw := map[resourceKind]string{
		resourceCPU:    overrides.CPU,
		resourceMemory: overrides.Memory,
	}
	for _, kind := range []resourceKind{resourceCPU, resourceMemory} {
		if raw[kind] == "" {
			continue
		}
		q, perr := resource.ParseQuantity(raw[kind])
		if perr != nil {
			input.Logger.Error("autotune: cannot parse resourcesRequests override, leaving the measurement untouched",
				"resource", kind, "value", raw[kind], "error", perr)
			setAutotuneDegraded(input, autotuneDegradedMetricGroup, degradedReasonBadOverride)
			out[kind] = measurementOverride{}
			continue
		}
		v := q.Value()
		if kind == resourceCPU {
			v = q.MilliValue()
		}
		out[kind] = measurementOverride{known: true, set: true, value: v}
	}
	return out, usedGlobal
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

	// RFC3339, UTC. The hook is woken by events as well as by cron; this is what
	// keeps raise/lower inside the daily window.
	LastMetricsRun string `json:"lastMetricsRun,omitempty"`
	// Normalized to millicpu or bytes — as a raw string "2" and "2000m" would look
	// like a change every run. nil means the measurement is under autotune.
	AppliedOverride *int64 `json:"appliedOverride,omitempty"`
}

type autotuneState map[resourceKind]*autotuneMeasurementState

// Carried unparsed because the filter runs inside the informer, where an error
// aborts snapshot creation and the hook never runs at all.
type autotuneStateRaw struct {
	State string `json:"state,omitempty"`
}

func applyAutotuneStateFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	cm := &v1.ConfigMap{}
	if err := sdk.FromUnstructured(obj, cm); err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}
	return autotuneStateRaw{State: cm.Data[autotuneStateKey]}, nil
}

func parseAutotuneState(raw string) (autotuneState, error) {
	if raw == "" {
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
	var raw autotuneStateRaw
	if err := snapshots[0].UnmarshalTo(&raw); err != nil {
		return nil, fmt.Errorf("unmarshal AutotuneState snapshot: %w", err)
	}
	return parseAutotuneState(raw.State)
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
			cpu, mem := sumContainerRequests(pod.Spec.Containers)
			initCPU, initMem := sumContainerRequests(pod.Spec.InitContainers)
			milliCPU += cpu + initCPU
			memoryBytes += mem + initMem
		}
		out[name] = nodeOtherRequests{MilliCPU: milliCPU, MemoryBytes: memoryBytes}
	}
	return out, nil
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
		setAppliedIfChanged(m, comp, resourceName, fallbackSplit(combinedBudget, componentMeta[comp].percent), ts)
	}
}

// ensureFullComponents fills missing applied* slots so the CM always carries a
// complete components map. Measurements skipped above are filled too — skipping
// them preserves existing values, and an empty slot has nothing to preserve.
// A measurement with no budget is left out: zeroes would be worse than nothing.
func ensureFullComponents(state autotuneState, combined map[resourceKind]int64, now time.Time) {
	ts := now.Format(time.RFC3339)
	for _, kind := range []resourceKind{resourceCPU, resourceMemory} {
		if combined[kind] <= 0 {
			continue
		}
		m := ensureMeasurement(state, kind)
		for _, comp := range controlPlaneComponents {
			if appliedValue(m.Components[comp], kind) > 0 {
				continue
			}
			setAppliedIfChanged(m, comp, kind, fallbackSplit(combined[kind], componentMeta[comp].percent), ts)
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
	input *go_hook.HookInput,
	dc dependency.Container,
	fetch func(context.Context, dependency.Container, string, resourceKind) (float64, bool, error),
	resourceName resourceKind,
	nodeBudget int64,
) (map[string]int64, bool) {
	recs := make(map[string]int64, len(controlPlaneComponents))
	usageOK := true
	for _, comp := range controlPlaneComponents {
		v, ok, ferr := fetch(ctx, dc, comp, resourceName)
		if ferr != nil {
			usageOK = false
			input.Logger.Warn("autotune: metrics API fetch failed", "resource", resourceName, "component", comp, "error", ferr)
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

	container := componentMeta[component].container
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
	// Anti-flap cooldowns — Go constants, not config-values.
	raiseCooldown = 5 * time.Hour
	lowerCooldown = 15 * time.Hour
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
	return fallbackSplit(combined, componentMeta[comp].percent)
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
		if !setAppliedIfChanged(m, comp, resourceName, proposed[comp], ts) {
			continue
		}
		input.Logger.Info("autotune: committed recommendation",
			"component", comp, "resource", resourceName, "action", action, "value", proposed[comp])
	}

	return deficit
}
