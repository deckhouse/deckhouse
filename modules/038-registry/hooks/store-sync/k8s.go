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

package storesync

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

const storeSyncSnap = "store-sync-job"

// KubernetesConfig returns the snapshot config for the store-sync Job.
func KubernetesConfig() go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:              storeSyncSnap,
		ApiVersion:        "batch/v1",
		Kind:              "Job",
		NamespaceSelector: helpers.NamespaceSelector,
		NameSelector:      &types.NameSelector{MatchNames: []string{"registry-cache-store-sync"}},
		FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
			var job batchv1.Job
			if err := sdk.FromUnstructured(obj, &job); err != nil {
				return nil, fmt.Errorf("convert Job %q: %w", obj.GetName(), err)
			}
			return int(job.Status.Succeeded), nil
		},
	}
}
