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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

// Synchronization / OnBeforeHelm / Node events: repopulate values and recheck
// capacityBlocked against the current fit budget (no metrics API).
// Other-pod requests are listed per master (fieldSelector spec.nodeName).
// Daily cron in resources_requests_autotune.go owns raise/lower decisions.
// OnBeforeHelm clears manual overrides / repopulates components without waiting
// for cron and without querying Prometheus on every ModuleRun.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        autotuneQueue,
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		controlPlaneNodesBinding(true, true),
		autotuneStateBinding(true),
	},
}, dependency.WithExternalDependencies(func(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	return runAutotune(ctx, input, dc, false)
}))
