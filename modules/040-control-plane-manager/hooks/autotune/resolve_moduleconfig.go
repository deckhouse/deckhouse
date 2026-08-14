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

package autotune

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

// Two links are instances of this: the module's own config and the deprecated
// location in the global one.
type moduleConfigResolver struct {
	snapshotName string
	source       requestsSource
	state        autotuneState
	fallback     requestsResolver
}

func (r *moduleConfigResolver) resolve(ctx context.Context, deps resolveDeps, kind resourceKind) (resolvedRequests, error) {
	deps.input.Logger.Debug("autotune: entering chain link", "resource", kind, "link", r.source)

	configured, err := readModuleConfigRequests(deps.input, r.snapshotName)
	if err != nil {
		return resolvedRequests{}, degraded(degradedReasonBadOverride, err)
	}

	budget, isSet, err := configured.budget(kind)
	if err != nil {
		return resolvedRequests{}, degraded(degradedReasonBadOverride, fmt.Errorf("%s: %w", r.snapshotName, err))
	}
	if !isSet {
		return r.fallback.resolve(ctx, deps, kind)
	}

	// The marker the dynamic link reads to notice a removed override.
	measurement := r.state.getOrCreateMeasurement(kind)
	measurement.AppliedOverride = ptr.To(budget)

	return resolvedRequests{
		byComponent: splitAcrossComponents(budget),
		source:      r.source,
	}, nil
}

type moduleConfigResourcesRequests struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

func readModuleConfigRequests(input *go_hook.HookInput, snapshotName string) (moduleConfigResourcesRequests, error) {
	snaps := input.Snapshots.Get(snapshotName)
	if len(snaps) == 0 {
		return moduleConfigResourcesRequests{}, nil
	}
	var configured moduleConfigResourcesRequests
	if err := snaps[0].UnmarshalTo(&configured); err != nil {
		return moduleConfigResourcesRequests{}, fmt.Errorf("unmarshal %s snapshot: %w", snapshotName, err)
	}
	return configured, nil
}

// Unset is the only case in which this link hands the kind to the automatics,
// hence a bool rather than a zero.
func (r moduleConfigResourcesRequests) budget(kind resourceKind) (int64, bool, error) {
	raw := r.CPU
	if kind == resourceMemory {
		raw = r.Memory
	}
	if raw == "" {
		return 0, false, nil
	}

	quantity, err := resource.ParseQuantity(raw)
	if err != nil {
		return 0, false, fmt.Errorf("parse %s resourcesRequests %q: %w", kind, raw, err)
	}

	value := quantity.Value()
	if kind == resourceCPU {
		value = quantity.MilliValue()
	}
	if value <= 0 {
		return 0, false, fmt.Errorf("%s resourcesRequests %q must be positive", kind, raw)
	}
	return value, true, nil
}
