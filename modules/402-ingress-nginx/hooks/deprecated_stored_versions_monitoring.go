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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	ingressNginxControllerCRDName                        = "ingressnginxcontrollers.deckhouse.io"
	ingressNginxControllerDeprecatedStoredVersionsMetric = "d8_ingress_nginx_controller_deprecated_stored_version"
	ingressNginxControllerStoredVersionsMetricsGroup     = "stored_versions"
	ingressNginxControllerDeprecatedStoredVersion        = "v1alpha1"
)

type ingressNginxControllerCRDStatus struct {
	Name           string
	StoredVersions []string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 11},
	Queue:        "/modules/ingress-nginx/monitoring",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "ingress_nginx_controller_crd",
			ApiVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
			NameSelector: &types.NameSelector{
				MatchNames: []string{ingressNginxControllerCRDName},
			},
			FilterFunc: applyIngressNginxControllerCRDStatusFilter,
		},
	},
}, monitorIngressNginxControllerStoredVersions)

func applyIngressNginxControllerCRDStatusFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var crd apiextensionsv1.CustomResourceDefinition
	if err := sdk.FromUnstructured(obj, &crd); err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	return ingressNginxControllerCRDStatus{
		Name:           crd.Name,
		StoredVersions: crd.Status.StoredVersions,
	}, nil
}

func monitorIngressNginxControllerStoredVersions(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(ingressNginxControllerStoredVersionsMetricsGroup)

	snapshots := input.Snapshots.Get("ingress_nginx_controller_crd")
	if len(snapshots) == 0 {
		return nil
	}

	var crdStatus ingressNginxControllerCRDStatus
	if err := snapshots[0].UnmarshalTo(&crdStatus); err != nil {
		return fmt.Errorf("failed to unmarshal 'ingress_nginx_controller_crd' snapshot: %w", err)
	}

	for _, version := range crdStatus.StoredVersions {
		if version != ingressNginxControllerDeprecatedStoredVersion {
			continue
		}

		input.MetricsCollector.Set(
			ingressNginxControllerDeprecatedStoredVersionsMetric,
			1,
			map[string]string{
				"crd":            crdStatus.Name,
				"stored_version": version,
			},
			metrics.WithGroup(ingressNginxControllerStoredVersionsMetricsGroup),
		)
	}

	return nil
}
