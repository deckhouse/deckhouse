/*
Copyright 2022 Flant JSC

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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	networkingv1 "k8s.io/api/networking/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const deprecatedIngressWithClientCertMetric = "d8_monitoring_ingress_with_client_cert"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/prometheus/deprecate_ingress_with_client_cert",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "main",
			Crontab: "*/5 * * * *",
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "ingress",
			ApiVersion: "networking.k8s.io/v1",
			Kind:       "Ingress",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-monitoring"},
				},
			},
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "heritage",
						Operator: "NotIn",
						Values:   []string{"deckhouse"},
					},
				},
			},
			FilterFunc: ingressWithClientCertFilter,
		},
	},
}, handleIngressWithClientCert)

type Ingress struct {
	Name string
}

func ingressWithClientCertFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ing = &networkingv1.Ingress{}
	err := sdk.FromUnstructured(obj, ing)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	confSnippet, ok := ing.Annotations["nginx.ingress.kubernetes.io/configuration-snippet"]
	if !ok {
		return nil, nil
	}

	isPrometheusIngress := false

	for _, rule := range ing.Spec.Rules {
		for _, path := range rule.HTTP.Paths {
			if strings.HasPrefix(path.Backend.Service.Name, "prometheus") {
				isPrometheusIngress = true
				break
			}
		}
	}

	if !isPrometheusIngress {
		return nil, nil
	}

	if strings.Contains(confSnippet, "/etc/nginx/ssl/client.crt") {
		return Ingress{
			Name: ing.Name,
		}, nil
	}

	return nil, nil
}

func handleIngressWithClientCert(_ context.Context, input *go_hook.HookInput) error {
	snaps := input.Snapshots.Get("ingress")

	if len(snaps) == 0 {
		return nil
	}

	for ing, err := range sdkobjectpatch.SnapshotIter[Ingress](snaps) {
		if err != nil {
			return fmt.Errorf("cannot iterate over ingress snapshot: %v", err)
		}

		input.MetricsCollector.Set(deprecatedIngressWithClientCertMetric, 1, map[string]string{
			"name": ing.Name,
		})
	}

	return nil
}
