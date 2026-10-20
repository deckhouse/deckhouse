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
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	ingressNginxAnnotationsVersionSnap = "ingress_nginx_annotations_version"
	ingressNginxPodsSnap               = "ingress_nginx_pods"

	ingressNginxNamespace = "d8-ingress-nginx"
	// ingressNginxAnnotationsVersionConfigMap is created by the ingress-nginx module with the generation of the
	// additional Deckhouse Ingress annotations its controllers support in data.version: the module release that
	// introduced it, such as v1.3.0. The module changes it only with new annotations, not with every release.
	ingressNginxAnnotationsVersionConfigMap = "additional-annotations-version"
	// ingressNginxAnnotationsVersionLabel is set by the ingress-nginx module on the controller and validator pods to
	// the generation of the annotations they support.
	ingressNginxAnnotationsVersionLabel = "ingress-nginx-controller.deckhouse.io/additional-annotations-version"

	ingressNginxAnnotationsVersionValuesPath = "global.discovery.ingressNginxAnnotationsVersion"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       ingressNginxAnnotationsVersionSnap,
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{ingressNginxNamespace}},
			},
			NameSelector:                 &types.NameSelector{MatchNames: []string{ingressNginxAnnotationsVersionConfigMap}},
			ExecuteHookOnEvents:          ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(true),
			FilterFunc:                   applyIngressNginxAnnotationsVersionFilter,
		},
		{
			Name:       ingressNginxPodsSnap,
			ApiVersion: "v1",
			Kind:       "Pod",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{ingressNginxNamespace}},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "app", Operator: metav1.LabelSelectorOpIn, Values: []string{"controller", "validator"}},
				},
			},
			ExecuteHookOnEvents:          ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(true),
			FilterFunc:                   applyIngressNginxPodFilter,
		},
	},
}, discoverIngressNginxAnnotationsVersion)

// ingressNginxPodVersion is a controller or validator pod with its annotations version label.
type ingressNginxPodVersion struct {
	Version string `json:"version"`
	// Finished is true for a pod that has terminated for good, which runs no version any more.
	Finished bool `json:"finished"`
}

func applyIngressNginxAnnotationsVersionFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	version, _, err := unstructured.NestedString(obj.Object, "data", "version")
	if err != nil {
		return nil, fmt.Errorf("get data.version: %w", err)
	}
	return version, nil
}

func applyIngressNginxPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	phase, _, err := unstructured.NestedString(obj.Object, "status", "phase")
	if err != nil {
		return nil, fmt.Errorf("get status.phase: %w", err)
	}
	return ingressNginxPodVersion{
		Version:  obj.GetLabels()[ingressNginxAnnotationsVersionLabel],
		Finished: phase == string(corev1.PodSucceeded) || phase == string(corev1.PodFailed),
	}, nil
}

// discoverIngressNginxAnnotationsVersion saves into the global discovery values the generation of the additional
// annotations the ingress-nginx controllers support: the minimum of the version in the additional-annotations-version
// ConfigMap of the module and the versions of all controller and validator pods that have not finished; without pods,
// the ConfigMap alone decides it. If the ConfigMap has no version or any version is not a semver, it is an empty string.
func discoverIngressNginxAnnotationsVersion(_ context.Context, input *go_hook.HookInput) error {
	versions, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, ingressNginxAnnotationsVersionSnap)
	if err != nil {
		return fmt.Errorf("unmarshal %s snapshot: %w", ingressNginxAnnotationsVersionSnap, err)
	}
	pods, err := sdkobjectpatch.UnmarshalToStruct[ingressNginxPodVersion](input.Snapshots, ingressNginxPodsSnap)
	if err != nil {
		return fmt.Errorf("unmarshal %s snapshot: %w", ingressNginxPodsSnap, err)
	}

	version := ""
	if len(versions) > 0 {
		version = versions[0]
	}
	input.Values.Set(ingressNginxAnnotationsVersionValuesPath, ingressNginxVersion(version, pods))
	return nil
}

// ingressNginxVersion returns the minimum of version and the versions of the pods that have not finished, or an empty
// string if version or any pod version is not a semver.
func ingressNginxVersion(version string, pods []ingressNginxPodVersion) string {
	if version == "" {
		return ""
	}

	minVersion, err := semver.NewVersion(version)
	if err != nil {
		return ""
	}

	for _, pod := range pods {
		if pod.Finished {
			continue
		}

		podVersion, err := semver.NewVersion(pod.Version)
		if err != nil {
			return ""
		}

		if podVersion.LessThan(minVersion) {
			minVersion = podVersion
		}
	}

	return minVersion.Original()
}
