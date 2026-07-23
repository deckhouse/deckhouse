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
	"io"
	"math"
	"net/http"
	"net/url"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/set"
)

const (
	autotuneScheduleName = "autotune"
	autotuneQueue        = "/modules/control-plane-manager/autotune"

	autotuneStateCMName = "d8-control-plane-manager-resources-autotune-state"
	autotuneStateKey    = "state"

	autotuneMetricName  = "d8_control_plane_manager_resources_autotune_insufficient_capacity"
	autotuneMetricGroup = "D8ControlPlaneResourcesAutotuneInsufficientCapacity"

	// Anti-flap calibration — Go constants, not config-values.
	// DEBUG timings (restore before production — see PR/commit notes):
	//   lookbackWindow: 7m (PodMetric PromQL)  ← prod 7d
	//   cron:           */5 * * * *            ← prod "0 3 * * *"
	//   raiseCooldown:  5 * time.Minute        ← prod 24 * time.Hour
	//   lowerCooldown:  15 * time.Minute       ← prod 72 * time.Hour
	// lookbackWindow is baked into PodMetric PromQL in
	// templates/podmetrics-autotune.yaml and must stay in sync.
	raiseThreshold      = 0.20 // +20%
	lowerThreshold      = 0.30 // −30%
	raiseCooldown       = 5 * time.Minute
	lowerCooldown       = 15 * time.Minute
	autotuneMinMilliCPU = int64(10)
	autotuneMinMemory   = int64(15 * 1024 * 1024) // 15 MiB
)

// fetchComponentUsage reads lookback-average usage for a control-plane component from
// the custom.metrics.k8s.io API (served by prometheus-metrics-adapter via PodMetric CRs).
// Returns (value, ok, err); ok=false means no usable datapoint (cold start / missing series).
// Overridable in unit tests.
var fetchComponentUsage = fetchComponentUsageFromMetricsAPI

type autotuneComponentState struct {
	AppliedMilliCPU *int64 `json:"appliedMilliCPU,omitempty"`
	AppliedBytes    *int64 `json:"appliedBytes,omitempty"`
	LastChange      string `json:"lastChange,omitempty"`
}

type capacityBlocked struct {
	Since   string `json:"since"`
	Deficit int64  `json:"deficit"`
}

type autotuneMeasurementState struct {
	Components      map[string]autotuneComponentState `json:"components,omitempty"`
	CapacityBlocked *capacityBlocked                  `json:"capacityBlocked,omitempty"`
}

// autotuneState nests by measurement (cpu/memory) so a manual override can delete
// a whole measurement branch for all four components in one patch.
type autotuneState struct {
	CPU    *autotuneMeasurementState `json:"cpu,omitempty"`
	Memory *autotuneMeasurementState `json:"memory,omitempty"`
}

func (s *autotuneState) measurement(resourceName string) *autotuneMeasurementState {
	switch resourceName {
	case resourceCPU:
		return s.CPU
	case resourceMemory:
		return s.Memory
	default:
		return nil
	}
}

func (s *autotuneState) setMeasurement(resourceName string, m *autotuneMeasurementState) {
	switch resourceName {
	case resourceCPU:
		s.CPU = m
	case resourceMemory:
		s.Memory = m
	}
}

func (s *autotuneState) deleteMeasurement(resourceName string) {
	s.setMeasurement(resourceName, nil)
}

func applyAutotuneStateFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	cm := &v1.ConfigMap{}
	if err := sdk.FromUnstructured(obj, cm); err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}
	raw, ok := cm.Data[autotuneStateKey]
	if !ok || raw == "" {
		return &autotuneState{}, nil
	}
	var st autotuneState
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		return nil, fmt.Errorf("unmarshal autotune state: %w", err)
	}
	return &st, nil
}

func readAutotuneState(input *go_hook.HookInput) (*autotuneState, error) {
	snapshots := input.Snapshots.Get("AutotuneState")
	if len(snapshots) == 0 {
		return &autotuneState{}, nil
	}
	var st autotuneState
	if err := snapshots[0].UnmarshalTo(&st); err != nil {
		return nil, fmt.Errorf("unmarshal AutotuneState snapshot: %w", err)
	}
	return &st, nil
}

func autotuneNodesBinding(onSync bool) go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:       "NodesResources",
		ApiVersion: "v1",
		Kind:       "Node",
		LabelSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      "node-role.kubernetes.io/control-plane",
				Operator: metav1.LabelSelectorOpExists,
			},
		}},
		FilterFunc:                   applyNodesResourcesFilter,
		ExecuteHookOnEvents:          ptr.To(false),
		ExecuteHookOnSynchronization: ptr.To(onSync),
	}
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

// Daily schedule path: full evaluation cycle.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: autotuneQueue,
	Schedule: []go_hook.ScheduleConfig{
		{Name: autotuneScheduleName, Crontab: "*/5 * * * *"}, // DEBUG: prod "0 3 * * *"
	},
	Kubernetes: []go_hook.KubernetesConfig{
		autotuneNodesBinding(false),
		autotuneStateBinding(false),
	},
}, dependency.WithExternalDependencies(autotuneResourcesRequestsSchedule))

func autotuneResourcesRequestsSchedule(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	return runAutotune(ctx, input, dc, true)
}

func runAutotune(ctx context.Context, input *go_hook.HookInput, dc dependency.Container, schedule bool) error {
	nodes, err := sdkobjectpatch.UnmarshalToStruct[Node](input.Snapshots, "NodesResources")
	if err != nil {
		return fmt.Errorf("unmarshal NodesResources snapshots: %w", err)
	}
	// Managed cloud — no master Nodes visible; leave combined-budget hook alone.
	if len(nodes) == 0 {
		return nil
	}

	state, err := readAutotuneState(input)
	if err != nil {
		return err
	}
	stateDirty := false

	cpuOverridden := isMeasurementOverridden(input, resourceCPU)
	memoryOverridden := isMeasurementOverridden(input, resourceMemory)

	if cpuOverridden && state.CPU != nil {
		state.deleteMeasurement(resourceCPU)
		stateDirty = true
	}
	if memoryOverridden && state.Memory != nil {
		state.deleteMeasurement(resourceMemory)
		stateDirty = true
	}

	pmaEnabled := set.NewFromValues(input.Values, "global.enabledModules").Has("prometheus-metrics-adapter")

	budgetCPU, budgetMem, _ := minMasterNodeBudget(nodes)
	combinedCPU := input.Values.Get("controlPlaneManager.internal.resourcesRequests.milliCpuControlPlane").Int()
	combinedMem := input.Values.Get("controlPlaneManager.internal.resourcesRequests.memoryControlPlane").Int()

	// Always rebuild components from state (idempotent repopulate).
	repopulateComponents(input, state, cpuOverridden, memoryOverridden)

	input.MetricsCollector.Expire(autotuneMetricGroup)
	emitCapacityBlockedMetrics(input, state)

	if !schedule {
		if stateDirty {
			return persistAutotuneState(input, state)
		}
		return nil
	}

	// Schedule path: evaluate only when PMA is enabled. Without it keep applied
	// state and re-emitted alert markers.
	if !pmaEnabled {
		if stateDirty {
			return persistAutotuneState(input, state)
		}
		return nil
	}

	now := dc.GetClock().Now().UTC()
	usageOK := true
	recsCPU := make(map[string]int64, len(controlPlaneComponents))
	recsMem := make(map[string]int64, len(controlPlaneComponents))

	for _, comp := range controlPlaneComponents {
		if !cpuOverridden {
			v, ok, ferr := fetchComponentUsage(ctx, dc, comp, resourceCPU)
			if ferr != nil {
				input.Logger.Warn("autotune: metrics API cpu fetch failed", "component", comp, "error", ferr)
				usageOK = false
			} else if ok {
				recsCPU[comp] = clampRecommendation(v, resourceCPU, budgetCPU)
			}
		}
		if !memoryOverridden {
			v, ok, ferr := fetchComponentUsage(ctx, dc, comp, resourceMemory)
			if ferr != nil {
				input.Logger.Warn("autotune: metrics API memory fetch failed", "component", comp, "error", ferr)
				usageOK = false
			} else if ok {
				recsMem[comp] = clampRecommendation(v, resourceMemory, budgetMem)
			}
		}
	}

	// Missing/failed metrics: do not mutate applied*; keep capacityBlocked as-is
	// (alert re-emitted above). Still persist override-driven deletions.
	if !usageOK && len(recsCPU) == 0 && len(recsMem) == 0 {
		if stateDirty {
			return persistAutotuneState(input, state)
		}
		return nil
	}

	if !cpuOverridden {
		changed, err := evaluateMeasurement(input, state, resourceCPU, recsCPU, budgetCPU, combinedCPU, now)
		if err != nil {
			return err
		}
		stateDirty = stateDirty || changed
	}
	if !memoryOverridden {
		changed, err := evaluateMeasurement(input, state, resourceMemory, recsMem, budgetMem, combinedMem, now)
		if err != nil {
			return err
		}
		stateDirty = stateDirty || changed
	}

	// Re-emit after evaluate (capacityBlocked may have changed).
	input.MetricsCollector.Expire(autotuneMetricGroup)
	emitCapacityBlockedMetrics(input, state)
	repopulateComponents(input, state, cpuOverridden, memoryOverridden)

	if stateDirty {
		return persistAutotuneState(input, state)
	}
	return nil
}

func isMeasurementOverridden(input *go_hook.HookInput, resourceName string) bool {
	switch resourceName {
	case resourceCPU:
		return input.Values.Exists("controlPlaneManager.resourcesRequests.cpu") ||
			input.Values.Exists("global.modules.resourcesRequests.controlPlane.cpu")
	case resourceMemory:
		return input.Values.Exists("controlPlaneManager.resourcesRequests.memory") ||
			input.Values.Exists("global.modules.resourcesRequests.controlPlane.memory")
	default:
		return false
	}
}

func repopulateComponents(input *go_hook.HookInput, state *autotuneState, cpuOverridden, memoryOverridden bool) {
	components := map[string]any{}
	for _, comp := range controlPlaneComponents {
		entry := map[string]any{}
		if !cpuOverridden {
			if m := state.measurement(resourceCPU); m != nil {
				if cs, ok := m.Components[comp]; ok && cs.AppliedMilliCPU != nil {
					entry["milliCPU"] = *cs.AppliedMilliCPU
				}
			}
		}
		if !memoryOverridden {
			if m := state.measurement(resourceMemory); m != nil {
				if cs, ok := m.Components[comp]; ok && cs.AppliedBytes != nil {
					entry["memoryBytes"] = *cs.AppliedBytes
				}
			}
		}
		if len(entry) > 0 {
			components[comp] = entry
		}
	}

	if len(components) == 0 {
		input.Values.Remove("controlPlaneManager.internal.resourcesRequests.components")
		return
	}
	// Set the whole map so JSON-patch does not need intermediate parents.
	input.Values.Set("controlPlaneManager.internal.resourcesRequests.components", components)
}

func emitCapacityBlockedMetrics(input *go_hook.HookInput, state *autotuneState) {
	for _, res := range []string{resourceCPU, resourceMemory} {
		m := state.measurement(res)
		if m == nil || m.CapacityBlocked == nil {
			continue
		}
		input.MetricsCollector.Set(
			autotuneMetricName,
			float64(m.CapacityBlocked.Deficit),
			map[string]string{"resource": res},
			metrics.WithGroup(autotuneMetricGroup),
		)
	}
}

type decideAction int

const (
	decideSkip decideAction = iota
	decideRaise
	decideLower
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

func clampRecommendation(raw float64, resourceName string, nodeBudget int64) int64 {
	var v int64
	switch resourceName {
	case resourceCPU:
		// PromQL returns cores; convert to millicpu.
		v = int64(math.Ceil(raw * 1000))
		if v < autotuneMinMilliCPU {
			v = autotuneMinMilliCPU
		}
	case resourceMemory:
		v = int64(math.Ceil(raw))
		if v < autotuneMinMemory {
			v = autotuneMinMemory
		}
	}
	if nodeBudget > 0 && v > nodeBudget {
		v = nodeBudget
	}
	return v
}

func evaluateMeasurement(
	input *go_hook.HookInput,
	state *autotuneState,
	resourceName string,
	recs map[string]int64,
	nodeBudget int64,
	combinedBudget int64,
	now time.Time,
) (bool, error) {
	if len(recs) == 0 {
		return false, nil
	}

	m := state.measurement(resourceName)
	if m == nil {
		m = &autotuneMeasurementState{Components: map[string]autotuneComponentState{}}
		state.setMeasurement(resourceName, m)
	}
	if m.Components == nil {
		m.Components = map[string]autotuneComponentState{}
	}

	proposed := make(map[string]int64, len(controlPlaneComponents))
	actions := make(map[string]decideAction, len(controlPlaneComponents))
	anyRaise := false

	for _, comp := range controlPlaneComponents {
		applied := appliedValue(m.Components[comp], resourceName)
		if applied == 0 {
			applied = fallbackSplit(combinedBudget, componentFallbackPercent[comp])
		}
		proposed[comp] = applied

		rec, hasRec := recs[comp]
		if !hasRec {
			actions[comp] = decideSkip
			continue
		}

		lastChange := parseLastChange(m.Components[comp].LastChange)
		action := decide(rec, appliedValue(m.Components[comp], resourceName), lastChange, now)
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
					proposed[comp] = appliedOrFallback(m.Components[comp], resourceName, combinedBudget, comp)
					actions[comp] = decideSkip
				}
			}
			if m.CapacityBlocked == nil {
				m.CapacityBlocked = &capacityBlocked{Since: now.Format(time.RFC3339), Deficit: deficit}
			} else {
				m.CapacityBlocked.Deficit = deficit
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
		val := proposed[comp]
		switch resourceName {
		case resourceCPU:
			cs.AppliedMilliCPU = ptr.To(val)
		case resourceMemory:
			cs.AppliedBytes = ptr.To(val)
		}
		cs.LastChange = now.Format(time.RFC3339)
		m.Components[comp] = cs
		changed = true
		input.Logger.Info("autotune: committed recommendation",
			"component", comp, "resource", resourceName, "action", actionName(action), "value", val)
	}

	return changed, nil
}

func actionName(a decideAction) string {
	switch a {
	case decideRaise:
		return "raise"
	case decideLower:
		return "lower"
	default:
		return "skip"
	}
}

func appliedValue(cs autotuneComponentState, resourceName string) int64 {
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

func appliedOrFallback(cs autotuneComponentState, resourceName string, combined int64, comp string) int64 {
	if v := appliedValue(cs, resourceName); v > 0 {
		return v
	}
	return fallbackSplit(combined, componentFallbackPercent[comp])
}

func parseLastChange(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func persistAutotuneState(input *go_hook.HookInput, state *autotuneState) error {
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

	gvks, _, err := scheme.Scheme.ObjectKinds(cm)
	if err == nil && len(gvks) > 0 {
		cm.SetGroupVersionKind(gvks[0])
	}

	input.PatchCollector.CreateOrUpdate(cm)
	return nil
}

// customMetricValueList is the subset of custom.metrics.k8s.io MetricValueList we need.
type customMetricValueList struct {
	Items []struct {
		Value string `json:"value"`
	} `json:"items"`
}

func podMetricName(component, resourceName string) string {
	container := componentContainer[component]
	return fmt.Sprintf("d8-cpm-autotune-%s-%s", container, resourceName)
}

func fetchComponentUsageFromMetricsAPI(ctx context.Context, dc dependency.Container, component, resourceName string) (float64, bool, error) {
	container := componentContainer[component]
	metric := podMetricName(component, resourceName)
	selector := url.QueryEscape(fmt.Sprintf("component=%s,tier=control-plane", container))
	requestURI := fmt.Sprintf(
		"/apis/custom.metrics.k8s.io/v1beta1/namespaces/%s/pods/*/%s?labelSelector=%s",
		kubeSystemNS, metric, selector,
	)

	body, err := rawGetCustomMetrics(ctx, dc, requestURI)
	if err != nil {
		return 0, false, err
	}

	var list customMetricValueList
	if err := json.Unmarshal(body, &list); err != nil {
		return 0, false, fmt.Errorf("decode metrics response: %w", err)
	}
	if len(list.Items) == 0 {
		return 0, false, nil
	}

	var maxVal float64
	found := false
	for _, item := range list.Items {
		q, err := resource.ParseQuantity(item.Value)
		if err != nil {
			continue
		}
		var v float64
		switch resourceName {
		case resourceCPU:
			v = float64(q.MilliValue()) / 1000.0
		case resourceMemory:
			v = float64(q.Value())
		}
		if math.IsNaN(v) || v < 0 {
			continue
		}
		if !found || v > maxVal {
			maxVal = v
			found = true
		}
	}
	return maxVal, found, nil
}

// rawGetCustomMetrics GETs a custom.metrics request URI.
//
// client-go's rest.Request and net/url.EscapedPath turn path '*' into '%2A',
// but custom.metrics.k8s.io only accepts a literal '*' as the "all objects"
// wildcard (kubectl --raw works; AbsPath/Do does not). Use url.URL.Opaque so
// the path is sent unescaped.
func rawGetCustomMetrics(ctx context.Context, dc dependency.Container, requestURI string) ([]byte, error) {
	config, err := dc.GetClientConfig()
	if err != nil {
		return nil, fmt.Errorf("get client config: %w", err)
	}
	httpClient, err := rest.HTTPClientFor(config)
	if err != nil {
		return nil, fmt.Errorf("http client: %w", err)
	}

	base, err := url.Parse(config.Host)
	if err != nil {
		return nil, fmt.Errorf("parse host: %w", err)
	}
	rel, err := url.Parse(requestURI)
	if err != nil {
		return nil, fmt.Errorf("parse request uri: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		return nil, err
	}
	// Opaque is written as-is by RequestURI(); Path/RawPath would escape '*'.
	req.URL = &url.URL{
		Scheme:   base.Scheme,
		Opaque:   "//" + base.Host + rel.Path,
		RawQuery: rel.RawQuery,
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", requestURI, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("GET %s: read body: %w", requestURI, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s: %s: %s", requestURI, resp.Status, string(body))
	}
	return body, nil
}
