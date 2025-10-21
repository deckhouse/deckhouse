/*
Copyright 2021 Flant JSC

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

package smokemini

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/modules/500-upmeter/hooks/smokemini/internal/snapshot"
)

// Scrape smoke-mini statefulset state before Helm rendering to avoid statefulset re-creation
var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
		Queue:        "/modules/upmeter/update_selector",
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:              "statefulsets",
				ApiVersion:        "apps/v1",
				Kind:              "StatefulSet",
				NamespaceSelector: namespaceSelector,
				LabelSelector:     labelSelector,
				FilterFunc:        snapshot.NewStatefulSet,

				ExecuteHookOnEvents:          ptr.To(false),
				ExecuteHookOnSynchronization: ptr.To(false),
				// WaitForSynchronization:       ptr.To(false),
			},
		},
	},
	scrapeState,
)

func scrapeState(_ context.Context, input *go_hook.HookInput) error {
	if !smokeMiniEnabled(input.Values) {
		return nil
	}

	const statePath = "upmeter.internal.smokeMini.sts"
	// Parse the state from values
	statefulSets, err := sdkobjectpatch.UnmarshalToStruct[snapshot.StatefulSet](input.Snapshots, "statefulsets")
	if err != nil {
		return fmt.Errorf("failed to unmarshal statefulsets snapshot: %w", err)
	}
	state, err := parseState(input.Values.Get(statePath))
	if err != nil {
		return err
	}
	if state.Empty() {
		// Take care of the initial state. The values are the source of truth after they are
		// filled for the fist time.
		state.Populate(statefulSets)
		input.Values.Set(statePath, state)
	}

	return nil
}
