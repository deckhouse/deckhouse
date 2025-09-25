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

package hooks

import (
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
}, enableExtendedMonitoring)

func enableExtendedMonitoring(_ context.Context, input *go_hook.HookInput) error {
	d8SystemPatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]string{
				"extended-monitoring.deckhouse.io/enabled":      "",
				"prometheus.deckhouse.io/rules-watcher-enabled": "true",
			},
		},
	}
	input.PatchCollector.PatchWithMerge(d8SystemPatch, "v1", "Namespace", "", "d8-system")

	kubeSystemPatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]string{
				"extended-monitoring.deckhouse.io/enabled": "",
			},
		},
	}
	input.PatchCollector.PatchWithMerge(kubeSystemPatch, "v1", "Namespace", "", "kube-system")

	input.PatchCollector.PatchWithMutatingFunc(removeDeprecatedAnnotation, "v1", "Namespace", "", "d8-system")
	input.PatchCollector.PatchWithMutatingFunc(removeDeprecatedAnnotation, "v1", "Namespace", "", "kube-system")

	return nil
}

func removeDeprecatedAnnotation(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	objCopy := obj.DeepCopy()

	objCopyAnnotations := objCopy.GetAnnotations()
	delete(objCopyAnnotations, "extended-monitoring.flant.com/enabled")

	objCopy.SetAnnotations(objCopyAnnotations)

	return objCopy, nil
}
