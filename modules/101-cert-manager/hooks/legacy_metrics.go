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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/modules/101-cert-manager/hooks/internal"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue("metrics"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:          "certificate",
			ApiVersion:    "certmanager.k8s.io/v1alpha1",
			Kind:          "Certificate",
			LabelSelector: nonDeckhouseHeritageLabelSelector,
			FilterFunc:    applyLegacyCertManagerCRFilter,
		},
		{
			Name:          "cluster_issuer",
			ApiVersion:    "certmanager.k8s.io/v1alpha1",
			Kind:          "ClusterIssuer",
			LabelSelector: nonDeckhouseHeritageLabelSelector,
			FilterFunc:    applyLegacyCertManagerCRFilter,
		},
		{
			Name:                         "issuer",
			ApiVersion:                   "certmanager.k8s.io/v1alpha1",
			Kind:                         "Issuer",
			LabelSelector:                nonDeckhouseHeritageLabelSelector,
			ExecuteHookOnSynchronization: pointer.BoolPtr(true),
			FilterFunc:                   applyLegacyCertManagerCRFilter,
		},
		{
			Name:                         "ingress",
			ApiVersion:                   "networking.k8s.io/v1",
			Kind:                         "Ingress",
			LabelSelector:                nonDeckhouseHeritageLabelSelector,
			ExecuteHookOnSynchronization: pointer.BoolPtr(true),
			FilterFunc:                   applyLegacyIngressFilter,
		},
	},
}, legacyCertManagerCRMetrics)

const legacyMetrics = "cert-manager-legacy-cr"

func legacyCertManagerCRMetrics(input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(legacyMetrics)

	snap := input.Snapshots["certificate"]
	for _, obj := range snap {
		lobj := obj.(legacyObject)
		input.MetricsCollector.Add("d8_cert_manager_deprecated_resources", 1, map[string]string{"namespace": lobj.Namespace, "kind": "Certificate"}, metrics.WithGroup(legacyMetrics))
	}

	snap = input.Snapshots["cluster_issuer"]
	for _, obj := range snap {
		lobj := obj.(legacyObject)
		input.MetricsCollector.Add("d8_cert_manager_deprecated_resources", 1, map[string]string{"name": lobj.Name, "kind": "ClusterIssuer"}, metrics.WithGroup(legacyMetrics))
	}

	snap = input.Snapshots["issuer"]
	for _, obj := range snap {
		lobj := obj.(legacyObject)
		input.MetricsCollector.Add("d8_cert_manager_deprecated_resources", 1, map[string]string{"namespace": lobj.Namespace, "kind": "Issuer"}, metrics.WithGroup(legacyMetrics))
	}

	snap = input.Snapshots["ingress"]
	for _, obj := range snap {
		if obj == nil {
			continue
		}

		lobj := obj.(legacyObject)
		input.MetricsCollector.Add("d8_cert_manager_deprecated_resources", 1, map[string]string{"namespace": lobj.Namespace, "kind": "Ingress"}, metrics.WithGroup(legacyMetrics))
	}

	return nil
}

func applyLegacyIngressFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	annotations := obj.GetAnnotations()

	for key := range annotations {
		if strings.HasPrefix(key, "certmanager.k8s.io") {
			return legacyObject{
				Name:      obj.GetName(),
				Namespace: obj.GetNamespace(),
			}, nil
		}
	}

	return nil, nil
}

func applyLegacyCertManagerCRFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return legacyObject{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}, nil
}

type legacyObject struct {
	Name      string
	Namespace string
}

var nonDeckhouseHeritageLabelSelector = &metav1.LabelSelector{
	MatchExpressions: []metav1.LabelSelectorRequirement{
		{
			Key:      "heritage",
			Operator: metav1.LabelSelectorOpNotIn,
			Values:   []string{"deckhouse"},
		},
	},
}
