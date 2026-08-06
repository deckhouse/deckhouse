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

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

const (
	autotuneScheduleName = "autotune"
	autotuneQueue        = "/modules/control-plane-manager/autotune"
)

// Schedule entrypoint: metrics → decide → commit (daily cron only).
// OnBeforeHelm / sync live in resources_requests_autotune_sync.go so ModuleRun
// does not hit the metrics API.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: autotuneQueue,
	Schedule: []go_hook.ScheduleConfig{
		// DEBUG timings — restore production: "0 3 * * *"
		{Name: autotuneScheduleName, Crontab: "*/5 * * * *"},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		controlPlaneNodesBinding(false, false),
		autotuneStateBinding(false),
	},
}, dependency.WithExternalDependencies(func(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	return runAutotune(ctx, input, dc, true)
}))

// runAutotune runs the autotune path. When evaluate is true (schedule), it
// fetches metrics and may raise/lower. When false (sync / OnBeforeHelm), it
// only repopulates values and rechecks capacityBlocked.
func runAutotune(ctx context.Context, input *go_hook.HookInput, dc dependency.Container, evaluate bool) error {
	nodes, err := sdkobjectpatch.UnmarshalToStruct[Node](input.Snapshots, "NodesResources")
	if err != nil {
		return fmt.Errorf("unmarshal NodesResources snapshots: %w", err)
	}
	// Managed cloud — no master Nodes visible; leave combined-budget hook alone.
	if len(nodes) == 0 {
		return nil
	}

	// Without prometheus + PMA there are no PodMetric series. Do not keep frozen
	// per-component applied* values — fall back to the legacy combined-budget
	// split from resources_requests_calculate.go.
	if !controlPlaneAutotuneActive(input) {
		return discardAutotuneForLegacy(input)
	}

	state, err := readAutotuneState(input)
	if err != nil {
		return err
	}
	stateDirty := false

	overridden := map[resourceKind]bool{
		resourceCPU:    isMeasurementOverridden(input, resourceCPU),
		resourceMemory: isMeasurementOverridden(input, resourceMemory),
	}
	for _, kind := range []resourceKind{resourceCPU, resourceMemory} {
		if !overridden[kind] {
			continue
		}
		input.Logger.Info("autotune: measurement overridden by config, skipping", "resource", kind)
		if state[kind] != nil {
			delete(state, kind)
			stateDirty = true
		}
	}

	otherByNode, err := fetchOtherRequestsByMasterNodes(ctx, dc, nodes)
	if err != nil {
		return fmt.Errorf("list non-control-plane pod requests on masters: %w", err)
	}
	fitCPU, fitMemBytes, _ := minMasterFitBudget(nodes, otherByNode)
	fit := map[resourceKind]int64{resourceCPU: fitCPU, resourceMemory: fitMemBytes}
	combined := map[resourceKind]int64{
		resourceCPU:    input.Values.Get(pathMilliCPUControlPlane).Int(),
		resourceMemory: input.Values.Get(pathMemoryControlPlane).Int(),
	}
	now := dc.GetClock().Now().UTC()

	// Evaluate path: recommendations from metrics. Repopulate values exactly
	// once at the end — a second Remove of `components` fails merge when Exists
	// still sees the pre-patch snapshot.
	if evaluate {
		recs := map[resourceKind]map[string]int64{}
		usageOK := true
		for _, kind := range []resourceKind{resourceCPU, resourceMemory} {
			r, ok := fetchRecs(ctx, dc, fetchComponentUsage, kind, overridden[kind], fit[kind], func(comp string, ferr error) {
				input.Logger.Warn("autotune: metrics API fetch failed", "resource", kind, "component", comp, "error", ferr)
			})
			recs[kind] = r
			usageOK = usageOK && ok
		}

		plan := planInitialSnapshot(state, overridden[resourceCPU], overridden[resourceMemory], recs[resourceCPU], recs[resourceMemory])
		ready := map[resourceKind]bool{resourceCPU: plan.CPUReady, resourceMemory: plan.MemReady}
		initial := map[resourceKind]bool{resourceCPU: plan.InitialCPU, resourceMemory: plan.InitialMem}
		for _, kind := range []resourceKind{resourceCPU, resourceMemory} {
			if !initial[kind] || ready[kind] {
				continue
			}
			input.Logger.Info("autotune: waiting for complete recommendations before initial snapshot",
				"resource", kind, "have", len(recs[kind]), "need", len(controlPlaneComponents))
		}

		// Missing/failed metrics: do not mutate applied*; keep capacityBlocked as-is.
		// Evaluate each measurement independently — do NOT use `stateDirty || evaluate...`
		// or a successful cpu commit short-circuits and skips memory entirely.
		if usageOK || len(recs[resourceCPU]) > 0 || len(recs[resourceMemory]) > 0 {
			for _, kind := range []resourceKind{resourceCPU, resourceMemory} {
				if overridden[kind] || !ready[kind] {
					continue
				}
				if evaluateMeasurement(input, state, kind, recs[kind], fit[kind], combined[kind], now) {
					stateDirty = true
				}
				if initial[kind] && fillMissingAppliedFromFallback(state, kind, combined[kind]) {
					stateDirty = true
				}
			}
		}
	} else {
		// Node resource changes (node events / OnBeforeHelm): refresh the
		// capacity-blocked alert against the current fit budget without calling
		// the metrics API. Cron remains responsible for new raise decisions.
		for _, kind := range []resourceKind{resourceCPU, resourceMemory} {
			if overridden[kind] {
				continue
			}
			if recheckCapacityBlocked(state, kind, fit[kind], combined[kind], now) {
				stateDirty = true
			}
		}
	}

	input.MetricsCollector.Expire(autotuneMetricGroup)
	emitCapacityBlockedMetrics(input, state)
	repopulateComponents(input, state, overridden[resourceCPU], overridden[resourceMemory])

	if stateDirty {
		return persistAutotuneState(input, state)
	}
	return nil
}

const (
	autotuneStateCMName = "d8-control-plane-manager-resources-autotune-state"
	autotuneStateKey    = "state"

	autotuneMetricName  = "d8_control_plane_manager_resources_autotune_insufficient_capacity"
	autotuneMetricGroup = "D8ControlPlaneResourcesAutotuneInsufficientCapacity"
)

type autotuneComponentState struct {
	AppliedMilliCPU *int64 `json:"appliedMilliCPU,omitempty"`
	AppliedBytes    *int64 `json:"appliedBytes,omitempty"`
	LastChange      string `json:"lastChange,omitempty"`
}

type capacityBlocked struct {
	Since       string `json:"since"`
	Deficit     int64  `json:"deficit"`
	ProposedSum int64  `json:"proposedSum,omitempty"`
}

type autotuneMeasurementState struct {
	Components      map[string]autotuneComponentState `json:"components,omitempty"`
	CapacityBlocked *capacityBlocked                  `json:"capacityBlocked,omitempty"`
}

// autotuneState nests by measurement (cpu/memory) so a manual override can delete
// a whole measurement branch for all four components in one patch.
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
	snapshots := input.Snapshots.Get("AutotuneState")
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

func autotuneStateBinding(onSync bool) go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:       "AutotuneState",
		ApiVersion: "v1",
		Kind:       "ConfigMap",
		NamespaceSelector: &types.NamespaceSelector{
			NameSelector: &types.NameSelector{MatchNames: []string{kubeSystemNS}},
		},
		NameSelector: &types.NameSelector{
			MatchNames: []string{autotuneStateCMName},
		},
		FilterFunc:                   applyAutotuneStateFilter,
		ExecuteHookOnEvents:          ptr.To(false),
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

// fetchOtherRequestsByMasterNodes lists pods once per master (fieldSelector
// spec.nodeName) and sums requests of pods that are not autotuned control-plane
// static pods — O(masters) API calls instead of a cluster-wide pod informer.
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

// fillMissingAppliedFromFallback writes %-split baselines into empty applied*
// slots so the first values snapshot covers every component in one ModuleRun.
func fillMissingAppliedFromFallback(state autotuneState, resourceName resourceKind, combinedBudget int64) bool {
	if combinedBudget <= 0 {
		return false
	}
	m := ensureMeasurement(state, resourceName)
	changed := false
	for _, comp := range controlPlaneComponents {
		if appliedValue(m.Components[comp], resourceName) > 0 {
			continue
		}
		cs := m.Components[comp]
		setApplied(&cs, resourceName, fallbackSplit(combinedBudget, componentFallbackPercent[comp]))
		m.Components[comp] = cs
		changed = true
	}
	return changed
}

func repopulateComponents(input *go_hook.HookInput, state autotuneState, cpuOverridden, memoryOverridden bool) {
	components := map[string]any{}
	for _, comp := range controlPlaneComponents {
		entry := map[string]any{}
		if !cpuOverridden {
			if m := state[resourceCPU]; m != nil {
				if cs, ok := m.Components[comp]; ok && cs.AppliedMilliCPU != nil {
					entry["milliCPU"] = *cs.AppliedMilliCPU
				}
			}
		}
		if !memoryOverridden {
			if m := state[resourceMemory]; m != nil {
				if cs, ok := m.Components[comp]; ok && cs.AppliedBytes != nil {
					// Decimal string — Helm renders large float64 values in scientific notation.
					entry["memoryBytes"] = strconv.FormatInt(*cs.AppliedBytes, 10)
				}
			}
		}
		if len(entry) > 0 {
			components[comp] = entry
		}
	}
	if len(components) == 0 {
		if input.Values.Exists(pathComponents) {
			input.Values.Remove(pathComponents)
		}
		return
	}
	// Set the whole map so JSON-patch does not need intermediate parents.
	input.Values.Set(pathComponents, components)
}

func emitCapacityBlockedMetrics(input *go_hook.HookInput, state autotuneState) {
	for _, res := range []resourceKind{resourceCPU, resourceMemory} {
		m := state[res]
		if m == nil || m.CapacityBlocked == nil {
			continue
		}
		input.MetricsCollector.Set(
			autotuneMetricName,
			float64(m.CapacityBlocked.Deficit),
			map[string]string{"resource": string(res)},
			metrics.WithGroup(autotuneMetricGroup),
		)
	}
}

// discardAutotuneForLegacy clears per-component internal values and persistent
// autotune state so templates use the fixed %-split of milliCpuControlPlane /
// memoryControlPlane from the legacy calculate hook.
func discardAutotuneForLegacy(input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(autotuneMetricGroup)
	if input.Values.Exists(pathComponents) {
		input.Values.Remove(pathComponents)
	}
	// Only delete when the CM is in snapshots — otherwise every OnBeforeHelm /
	// schedule tick on clusters without PMA would spam Delete for a missing object.
	if len(input.Snapshots.Get("AutotuneState")) > 0 {
		input.Logger.Info("autotune: prometheus or prometheus-metrics-adapter disabled, discarding autotune and falling back to legacy combined budget")
		input.PatchCollector.Delete("v1", "ConfigMap", kubeSystemNS, autotuneStateCMName)
	}
	return nil
}

const (
	autotuneMinMilliCPU    = int64(10)
	autotuneMinMemoryBytes = int64(15 * 1000 * 1000) // 15M
)

// fetchComponentUsage is the production metrics client; overridable in unit tests.
var fetchComponentUsage = fetchComponentUsageFromMetricsAPI

func clampRecommendation(raw float64, resourceName resourceKind, nodeBudget int64) int64 {
	var v int64
	switch resourceName {
	case resourceCPU:
		// PromQL returns cores; convert to millicpu.
		v = int64(math.Ceil(raw * 1000))
		if v < autotuneMinMilliCPU {
			v = autotuneMinMilliCPU
		}
	case resourceMemory:
		// PodMetric returns plain bytes (rounded in PromQL).
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

// fetchRecs collects clamped recommendations for one measurement.
// On per-component fetch errors it sets usageOK=false and continues; missing
// series (ok=false, err=nil) simply omit that component from the map.
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

// customMetricValueList is the subset of custom.metrics.k8s.io MetricValueList we need.
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
		// Empty list is a real miss (PromQL returned no series). Surface it so
		// schedule logs show why memory/cpu was skipped instead of failing silently.
		return 0, false, fmt.Errorf("GET %s: empty MetricValueList", path)
	}

	// custom.metrics encodes samples as milli-quantities; AsApproximateFloat64
	// yields the natural unit (cores for cpu, bytes for memory — see PodMetric PromQL).
	v := list.Items[0].Value.AsApproximateFloat64()
	if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		return 0, false, fmt.Errorf("GET %s: non-positive metric value %q", path, list.Items[0].Value.String())
	}
	return v, true, nil
}

const (
	// Anti-flap cooldowns — Go constants, not config-values.
	// DEBUG timings — restore production: raise 24h, lower 72h.
	raiseCooldown = 5 * time.Minute
	lowerCooldown = 15 * time.Minute
)

type decideAction string

const (
	decideSkip  decideAction = "skip"
	decideRaise decideAction = "raise"
	decideLower decideAction = "lower"
)

// decide returns whether a recommendation should be committed given asymmetric
// deadband and cooldown. Pure function — covered by table tests.
func decide(rec, applied int64, lastChange, now time.Time) decideAction {
	if applied <= 0 {
		// First commit: treat as raise with no cooldown.
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

// effectiveApplied is the request currently rendered for a component: persisted
// applied* when present, otherwise the %-split of the combined budget.
func effectiveApplied(cs autotuneComponentState, resourceName resourceKind, combined int64, comp string) int64 {
	if v := appliedValue(cs, resourceName); v > 0 {
		return v
	}
	return fallbackSplit(combined, componentFallbackPercent[comp])
}

type initialSnapshotPlan struct {
	InitialCPU bool
	InitialMem bool
	CPUReady   bool
	MemReady   bool
}

// planInitialSnapshot decides whether each measurement may evaluate on this run.
// When both measurements need a first commit, both must have complete recs so
// cpu+memory land in one values write.
func planInitialSnapshot(
	state autotuneState,
	cpuOverridden, memoryOverridden bool,
	recsCPU, recsMem map[string]int64,
) initialSnapshotPlan {
	p := initialSnapshotPlan{
		InitialCPU: !cpuOverridden && !measurementHasAnyApplied(state[resourceCPU], resourceCPU),
		InitialMem: !memoryOverridden && !measurementHasAnyApplied(state[resourceMemory], resourceMemory),
	}
	p.CPUReady = !p.InitialCPU || len(recsCPU) == len(controlPlaneComponents)
	p.MemReady = !p.InitialMem || len(recsMem) == len(controlPlaneComponents)
	if p.InitialCPU && p.InitialMem && (!p.CPUReady || !p.MemReady) {
		p.CPUReady = false
		p.MemReady = false
	}
	return p
}

func evaluateMeasurement(
	input *go_hook.HookInput,
	state autotuneState,
	resourceName resourceKind,
	recs map[string]int64,
	nodeBudget int64,
	combinedBudget int64,
	now time.Time,
) bool {
	if len(recs) == 0 {
		input.Logger.Warn("autotune: no usage datapoints from metrics API, leaving state unchanged", "resource", resourceName)
		return false
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
		// Compare against the currently rendered request (applied* or %-split
		// fallback) so the first snapshot after clearing a manual override does
		// not unconditionally rewrite every component.
		action := decide(rec, eff, lastChange, now)
		actions[comp] = action
		if action == decideRaise || action == decideLower {
			proposed[comp] = rec
		}
		if action == decideRaise {
			anyRaise = true
		}
	}

	changed := false

	if anyRaise {
		var sum int64
		for _, comp := range controlPlaneComponents {
			sum += proposed[comp]
		}
		if sum > nodeBudget {
			deficit := sum - nodeBudget
			for _, comp := range controlPlaneComponents {
				if actions[comp] == decideRaise {
					proposed[comp] = effectiveApplied(m.Components[comp], resourceName, combinedBudget, comp)
					actions[comp] = decideSkip
				}
			}
			if m.CapacityBlocked == nil {
				m.CapacityBlocked = &capacityBlocked{
					Since:       now.Format(time.RFC3339),
					Deficit:     deficit,
					ProposedSum: sum,
				}
			} else {
				m.CapacityBlocked.Deficit = deficit
				m.CapacityBlocked.ProposedSum = sum
			}
			changed = true
			input.Logger.Info("autotune: raise blocked by capacity gate",
				"resource", resourceName, "deficit", deficit, "budget", nodeBudget, "proposedSum", sum)
		} else if m.CapacityBlocked != nil {
			m.CapacityBlocked = nil
			changed = true
		}
	} else if m.CapacityBlocked != nil {
		m.CapacityBlocked = nil
		changed = true
	}

	for _, comp := range controlPlaneComponents {
		action := actions[comp]
		if action == decideSkip {
			continue
		}
		cs := m.Components[comp]
		setApplied(&cs, resourceName, proposed[comp])
		cs.LastChange = now.Format(time.RFC3339)
		m.Components[comp] = cs
		changed = true
		input.Logger.Info("autotune: committed recommendation",
			"component", comp, "resource", resourceName, "action", action, "value", proposed[comp])
	}

	return changed
}

// recheckCapacityBlocked updates or clears capacityBlocked using the current fit
// budget (no metrics). Uses the last blocked ProposedSum when present so a node
// resize can expire the alert once the previously rejected total would fit.
func recheckCapacityBlocked(
	state autotuneState,
	resourceName resourceKind,
	fitBudget, combinedBudget int64,
	now time.Time,
) bool {
	m := state[resourceName]
	if m == nil {
		return false
	}
	var sumApplied int64
	for _, comp := range controlPlaneComponents {
		sumApplied += effectiveApplied(m.Components[comp], resourceName, combinedBudget, comp)
	}

	proposedSum := sumApplied
	if m.CapacityBlocked != nil && m.CapacityBlocked.ProposedSum > 0 {
		proposedSum = m.CapacityBlocked.ProposedSum
	}

	if proposedSum > fitBudget {
		deficit := proposedSum - fitBudget
		if m.CapacityBlocked == nil {
			m.CapacityBlocked = &capacityBlocked{
				Since:       now.Format(time.RFC3339),
				Deficit:     deficit,
				ProposedSum: proposedSum,
			}
			return true
		}
		if m.CapacityBlocked.Deficit == deficit && m.CapacityBlocked.ProposedSum == proposedSum {
			return false
		}
		m.CapacityBlocked.Deficit = deficit
		m.CapacityBlocked.ProposedSum = proposedSum
		return true
	}
	if m.CapacityBlocked != nil {
		m.CapacityBlocked = nil
		return true
	}
	return false
}
