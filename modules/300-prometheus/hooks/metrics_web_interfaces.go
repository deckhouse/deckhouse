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
	"errors"
	"path"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	netv1 "k8s.io/api/networking/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// This hook get all ingresses with labels `heritage: deckhouse` and  "deckhouse.io/export-domain"
// and generate metrics for the Grafana home page (table of all enabled deckhouse resources with web interface)
// you have to set name for resource via `deckhouse.io/export-domain` label for ingress, like:
//   deckhouse.io/export-domain: "prometheus"
//   deckhouse.io/export-domain: "cilium"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/prometheus/web_interfaces",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "ingresses",
			ApiVersion: "networking.k8s.io/v1",
			Kind:       "Ingress",
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"heritage": "deckhouse",
				},
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "deckhouse.io/export-domain",
						Operator: v1.LabelSelectorOpExists,
					},
				},
			},
			FilterFunc: filterIngress,
		},
	},
}, domainMetricHandler)

type exportedWebInterface struct {
	Name string
	URL  string
}

func filterIngress(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ing netv1.Ingress

	err := sdk.FromUnstructured(obj, &ing)
	if err != nil {
		return nil, err
	}

	name := ing.Labels["deckhouse.io/export-domain"]
	if name == "" {
		return nil, errors.New("exported domain name required")
	}

	if len(ing.Spec.Rules) == 0 {
		return nil, nil
	}

	rule := ing.Spec.Rules[0]

	host := rule.Host
	urlPath := ""
	if len(rule.HTTP.Paths) > 0 {
		urlPath = rule.HTTP.Paths[0].Path
	}

	if urlPath != "" {
		// cut path regexp replacements like /prometheus(/|$)(.*)
		// we don't need them for the main page
		index := strings.Index(urlPath, "(")
		if index > 0 {
			urlPath = urlPath[:index]
		}
	}

	return exportedWebInterface{
		Name: name,
		URL:  path.Join(host, urlPath),
	}, nil
}

func domainMetricHandler(input *go_hook.HookInput) error {
	snap := input.Snapshots["ingresses"]
	input.MetricsCollector.Expire("deckhouse_exported_domains")

	for _, sn := range snap {
		if sn == nil {
			continue
		}

		domain := sn.(exportedWebInterface)
		input.MetricsCollector.Set("deckhouse_web_interfaces", 1, map[string]string{"name": domain.Name, "url": domain.URL}, metrics.WithGroup("deckhouse_exported_domains"))
	}

	return nil
}
