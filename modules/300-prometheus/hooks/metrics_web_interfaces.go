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
	"fmt"
	"net/url"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	netv1 "k8s.io/api/networking/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

// This hook get all ingresses with labels `heritage: deckhouse` and 'module'
// then looking for "web.deckhouse.io/export-name" annotation
// and generate metrics for the Grafana home page (table of all enabled deckhouse resources with web interface)
// you have to set name for resource via `web.deckhouse.io/export-name` label for ingress, like:
//   web.deckhouse.io/export-name: "prometheus"
//   web.deckhouse.io/export-name: "cilium"
// Also you can set next annotations:
//   web.deckhouse.io/export-host - custom host for service, if not set - will take it from ingress rule
//   web.deckhouse.io/export-path - custom path for service; if not set - will take from ingress rule
//   web.deckhouse.io/export-icon - set custom icon for service (will be placed in grafana table), best choice - service favicon
//   icon could be one of the following:
//     1. Absolute URL to an icon (https://mycompany.com/icon/xxx.png)
//     2. Relative path to grafana container (/public/img/mylogo.png)
//     3. Data URI base64 string

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
						Key:      "module",
						Operator: v1.LabelSelectorOpExists,
					},
				},
			},
			FilterFunc: filterIngress,
		},
	},
}, domainMetricHandler)

type exportedWebInterface struct {
	Icon string
	Name string
	URL  string
}

func filterIngress(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ing netv1.Ingress

	err := sdk.FromUnstructured(obj, &ing)
	if err != nil {
		return nil, err
	}

	name := ing.Annotations["web.deckhouse.io/export-name"]
	if name == "" {
		return nil, nil
	}

	icon := ing.Annotations["web.deckhouse.io/export-icon"]
	if icon == "" {
		// unknown mark
		icon = "/public/img/unknown.png"
	}

	exportedScheme := "http"
	if len(ing.Spec.TLS) > 0 {
		exportedScheme = "https"
	}

	exportedHost := ing.Annotations["web.deckhouse.io/export-host"]

	exportedPath := ing.Annotations["web.deckhouse.io/export-path"]

	if exportedHost == "" {
		// label is not set, get it from spec
		if len(ing.Spec.Rules) == 0 {
			return nil, nil
		}

		rule := ing.Spec.Rules[0]
		exportedHost = rule.Host
	}

	if exportedPath == "" {
		if len(ing.Spec.Rules) == 0 {
			return nil, nil
		}

		rule := ing.Spec.Rules[0]

		if len(rule.HTTP.Paths) > 0 {
			exportedPath = rule.HTTP.Paths[0].Path
		}

		if exportedPath != "/" {
			// cut path regexp replacements like /prometheus(/|$)(.*)
			// we don't need them for the main page
			index := strings.Index(exportedPath, "(")
			if index > 0 {
				exportedPath = exportedPath[:index]
			}
		}
	}

	u := url.URL{
		Scheme: exportedScheme,
		Host:   exportedHost,
		Path:   exportedPath,
	}

	return exportedWebInterface{
		Icon: icon,
		Name: name,
		URL:  u.String(),
	}, nil
}

func domainMetricHandler(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire("deckhouse_exported_domains")
	globalHTTPSMode := input.ConfigValues.Get("global.modules.https.mode").String()

	for domain, err := range sdkobjectpatch.SnapshotIter[exportedWebInterface](input.Snapshots.Get("ingresses")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'ingresses' snapshots: %w", err)
		}

		if globalHTTPSMode == "OnlyInURI" {
			domain.URL = strings.ReplaceAll(domain.URL, "http://", "https://")
		}

		input.MetricsCollector.Set(
			"deckhouse_web_interfaces",
			1,
			map[string]string{
				"icon": domain.Icon,
				"name": domain.Name,
				"url":  domain.URL,
			},
			metrics.WithGroup("deckhouse_exported_domains"),
		)
	}

	return nil
}
