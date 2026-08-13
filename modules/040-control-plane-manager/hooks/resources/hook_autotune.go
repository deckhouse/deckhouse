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
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const (
	autotuneScheduleName = "autotune"
	autotuneQueue        = "/modules/control-plane-manager/autotune"
)

// Schedule → resolve per-component requests → ConfigMap.
// hook_sync.go projects that ConfigMap into values.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: autotuneQueue,
	Schedule: []go_hook.ScheduleConfig{
		{Name: autotuneScheduleName, Crontab: "0 3 * * *"},
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

// runAutotune resolves per-component requests through the resolver chain (see
// resolve.go) and persists them to the CM.
//
// The only error it may return is a failure to marshal its own state: any other
// error would make addon-operator drop the whole PatchCollector, and the
// control-plane would fall back onto the 512m/512Mi template default.
func runAutotune(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	input.MetricsCollector.Expire(autotuneDegradedMetricGroup)
	input.MetricsCollector.Expire(autotuneMetricGroup)
	input.MetricsCollector.Expire(obsoleteGlobalResourcesRequestsMetricGroup)

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

	state := readStateOrEmpty(input)
	resolver := newRequestsResolverChain(nodes, state)
	rctx := resolveContext{input: input, dc: dc}
	changedAt := dc.GetClock().Now().UTC().Format(time.RFC3339)

	// cpu and memory walk the same chain independently, which is what lets a
	// config that sets only one of them leave the other to the automatics.
	for _, kind := range []resourceKind{resourceCPU, resourceMemory} {
		resolved, err := resolver.resolve(ctx, rctx, kind)
		if err != nil {
			// The measurement is left exactly as it was: with no ConfigMap entry the
			// templates apply their own fallback, which beats an invented number. See
			// the contract on requestsResolver.
			input.Logger.Error("autotune: cannot resolve requests", "resource", kind, "error", err)
			setAutotuneDegraded(input, autotuneDegradedMetricGroup, degradedReasonOf(err))
			continue
		}

		if changed := state.commit(kind, resolved.byComponent, changedAt); len(changed) > 0 {
			input.Logger.Info("autotune: committed requests",
				"resource", kind, "components", changed, "source", resolved.source)
		}
		reportResolution(input, kind, resolved)
	}

	return persistAutotuneState(input, state)
}

// reportResolution surfaces what a resolution carries besides the values
// themselves: which link answered, and whether a measured raise was held back.
func reportResolution(input *go_hook.HookInput, kind resourceKind, resolved resolvedRequests) {
	if resolved.source == sourceGlobalConfig {
		// Configured in the deprecated location, global ModuleConfig.
		input.MetricsCollector.Set(
			obsoleteGlobalResourcesRequestsMetricName,
			1,
			map[string]string{},
			metrics.WithGroup(obsoleteGlobalResourcesRequestsMetricGroup),
		)
	}

	if resolved.deficit > 0 {
		input.Logger.Info("autotune: raise held back by the capacity gate",
			"resource", kind, "deficit", resolved.deficit)
		input.MetricsCollector.Set(
			autotuneMetricName,
			float64(resolved.deficit),
			map[string]string{"resource": string(kind)},
			metrics.WithGroup(autotuneMetricGroup),
		)
	}
}
