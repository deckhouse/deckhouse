// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	shtypes "github.com/flant/shell-operator/pkg/hook/types"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
)

// Unlocked is the set of a package's Kubernetes monitor ids whose events are unlocked: their
// synchronization finished, or UpdateMonitor recreated them unlocked. The owner guards it with the
// lock it holds around its hook controllers.
type Unlocked map[string]struct{}

// monitorGetter is the part of the kube events manager the snapshots dump reads.
type monitorGetter interface {
	GetMonitor(monitorID string) kubeeventsmanager.Monitor
}

// SnapshotsDump returns the snapshots of the bindings whose monitors are unlocked, by binding name,
// in the shape of shell-operator's SnapshotsDump. A locked monitor is skipped: reading its snapshot
// discards the events it buffers until its synchronization unlocks it (shell-operator
// resourceInformer.getCachedObjects). Each monitor is read once and may be gone.
func SnapshotsDump(bindings []shtypes.OnKubernetesEventConfig, monitors monitorGetter, unlocked Unlocked) map[string]any {
	dumps := make(map[string]any)

	for _, binding := range bindings {
		if binding.Monitor == nil {
			continue
		}

		id := binding.Monitor.Metadata.MonitorId
		if _, ok := unlocked[id]; !ok {
			continue
		}

		monitor := monitors.GetMonitor(id)
		if monitor == nil {
			continue
		}

		total, last := monitor.SnapshotOperations()
		dumps[binding.BindingName] = map[string]any{
			"snapshot": monitor.Snapshot(),
			"operations": map[string]any{
				"sinceStart":         total,
				"sinceLastExecution": last,
			},
		}
	}

	return dumps
}
