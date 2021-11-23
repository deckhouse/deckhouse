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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "annotated_ingress",
			ApiVersion:                   "networking.k8s.io/v1",
			Kind:                         "Ingress",
			LabelSelector:                nonDeckhouseHeritageLabelSelector,
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			ExecuteHookOnEvents:          pointer.BoolPtr(false),
			FilterFunc:                   applyLegacyAnnotatedIngressFilter,
		},
		{
			Name:       "migrated",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-cert-manager-migrated"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cert-manager"},
				},
			},
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			ExecuteHookOnEvents:          pointer.BoolPtr(false),
			FilterFunc:                   applyMigratedFilter,
		},
	},
}, handleLegacyAnnotatedIngress)

type ingressAnnotatedPatch struct {
	Name          string
	Namespace     string
	Annotations   map[string]string
	PatchRequired bool
}

func applyMigratedFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func applyLegacyAnnotatedIngressFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ingressPatch := &ingressAnnotatedPatch{
		Name:        obj.GetName(),
		Namespace:   obj.GetNamespace(),
		Annotations: obj.GetAnnotations(),
	}

	for annotation, value := range ingressPatch.Annotations {
		// Migration is based on
		// https://cert-manager.io/docs/installation/upgrading/upgrading-0.10-0.11/#additional-annotation-changes
		switch annotation {
		case "certmanager.k8s.io/acme-http01-edit-in-place":
			addIfNotExists(ingressPatch, "acme.cert-manager.io/http01-edit-in-place", value)

		case "certmanager.k8s.io/acme-http01-ingress-class":
			addIfNotExists(ingressPatch, "acme.cert-manager.io/http01-ingress-class", value)

		case "certmanager.k8s.io/issuer":
			addIfNotExists(ingressPatch, "cert-manager.io/issuer", value)

		case "certmanager.k8s.io/cluster-issuer":
			addIfNotExists(ingressPatch, "cert-manager.io/cluster-issuer", value)

		case "certmanager.k8s.io/alt-names":
			addIfNotExists(ingressPatch, "cert-manager.io/alt-names", value)

		case "certmanager.k8s.io/ip-sans":
			addIfNotExists(ingressPatch, "cert-manager.io/ip-sans", value)

		case "certmanager.k8s.io/common-name":
			addIfNotExists(ingressPatch, "cert-manager.io/common-name", value)

		case "certmanager.k8s.io/issuer-name":
			addIfNotExists(ingressPatch, "cert-manager.io/issuer-name", value)

		case "certmanager.k8s.io/issuer-kind":
			addIfNotExists(ingressPatch, "cert-manager.io/issuer-kind", value)
		default:
			continue
		}
	}

	return ingressPatch, nil
}

func addIfNotExists(obj *ingressAnnotatedPatch, key, value string) {
	if _, ok := obj.Annotations[key]; !ok {
		obj.Annotations[key] = value
		obj.PatchRequired = true
	}
}

func handleLegacyAnnotatedIngress(input *go_hook.HookInput) error {
	if len(input.Snapshots["migrated"]) > 0 {
		// We need this hook to run only once before starting the new cert-manager
		return nil
	}

	for _, obj := range input.Snapshots["annotated_ingress"] {
		ingress := obj.(*ingressAnnotatedPatch)

		if !ingress.PatchRequired {
			continue
		}

		annotationsPatch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": ingress.Annotations,
			},
		}

		input.PatchCollector.MergePatch(
			annotationsPatch,
			"networking.k8s.io/v1",
			"Ingress",
			ingress.Namespace,
			ingress.Name,
			object_patch.IgnoreMissingObject(),
		)
	}

	return nil
}
