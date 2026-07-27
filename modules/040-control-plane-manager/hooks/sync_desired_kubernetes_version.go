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
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

/*
Description:
	Writes the operator-declared desired Kubernetes version into
	spec.desiredVersion / spec.updateMode of the kube-system/d8-cluster-kubernetes ConfigMap.

	The declared target and Automatic-mode flag come from the global discovery hook
	(global.discovery.targetKubernetesVersion / kubernetesVersionIsAutomatic) — this hook does not
	resolve MC/CC itself and does not suppress writes on default-version drift (that signal is
	alert-only in the global resolver).

	The update-observer controller treats the ConfigMap spec block as external input: it
	reconciles status.* against it, but never computes spec.* itself. This hook is the single
	writer of spec.* on that ConfigMap — it never touches status.* or the k8s-version/
	max-k8s-version labels update-observer owns.
*/

const (
	desiredVersionConfigMapName      = "d8-cluster-kubernetes"
	desiredVersionConfigMapNamespace = "kube-system"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        moduleQueue,
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 15},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cluster_kubernetes_configmap",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NameSelector: &types.NameSelector{
				MatchNames: []string{desiredVersionConfigMapName},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{desiredVersionConfigMapNamespace},
				},
			},
			FilterFunc: sdkvFilterConfigMapSpec,
		},
	},
}, syncDesiredKubernetesVersion)

// configMapSpecSnapshot carries the ConfigMap's current data["spec"] (raw YAML, used to detect
// a no-op write). Read-only input; only "spec" is ever written back.
type configMapSpecSnapshot struct {
	Spec string
}

func sdkvFilterConfigMapSpec(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm corev1.ConfigMap
	if err := sdk.FromUnstructured(unstructured, &cm); err != nil {
		return nil, err
	}

	return configMapSpecSnapshot{Spec: cm.Data["spec"]}, nil
}

// configMapSpec mirrors the Spec struct update-observer's controller/configmap.go writes/reads
// for the ConfigMap's "spec" data key.
type configMapSpec struct {
	DesiredVersion string `json:"desiredVersion"`
	UpdateMode     string `json:"updateMode"`
}

func syncDesiredKubernetesVersion(_ context.Context, input *go_hook.HookInput) error {
	desired := input.Values.Get("global.discovery.targetKubernetesVersion").String()
	if desired == "" {
		// Global discovery has not published a target yet — nothing to write.
		return nil
	}

	updateMode := "Manual"
	if input.Values.Get("global.discovery.kubernetesVersionIsAutomatic").Bool() {
		updateMode = "Automatic"
	}

	var cm configMapSpecSnapshot
	if snaps, err := sdkobjectpatch.UnmarshalToStruct[configMapSpecSnapshot](input.Snapshots, "cluster_kubernetes_configmap"); err == nil && len(snaps) > 0 {
		cm = snaps[0]
	}

	specBytes, err := yaml.Marshal(configMapSpec{DesiredVersion: desired, UpdateMode: updateMode})
	if err != nil {
		return err
	}
	specYAML := string(specBytes)

	if cm.Spec == specYAML {
		return nil
	}

	// ConfigMap may not exist yet (e.g. before update-observer's DaemonSet pods have ever run) —
	// create it with just this one data key, then merge-patch the same key in case it already
	// existed with other keys/labels (owned by update-observer) that must not be touched.
	input.PatchCollector.CreateIfNotExists(&corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      desiredVersionConfigMapName,
			Namespace: desiredVersionConfigMapNamespace,
			Labels:    map[string]string{"heritage": "deckhouse"},
		},
		Data: map[string]string{"spec": specYAML},
	})

	patch := map[string]interface{}{
		"data": map[string]interface{}{
			"spec": specYAML,
		},
	}
	input.PatchCollector.PatchWithMerge(patch, "v1", "ConfigMap", desiredVersionConfigMapNamespace, desiredVersionConfigMapName, object_patch.WithIgnoreMissingObject())

	return nil
}
