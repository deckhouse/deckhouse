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
	"fmt"
	"strings"
	"unicode"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/tidwall/gjson"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/prometheus/deprecate_outdated_grafana_dashboard_crd",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "grafana_dashboard_definitions",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "GrafanaDashboardDefinition",
			FilterFunc: filterGrafanaDashboardCRD,
		},
	},
}, grafanaDashboardCRDsHandler)

func filterGrafanaDashboardCRD(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	definition, ok, err := unstructured.NestedString(obj.Object, "spec", "definition")
	if err != nil {
		return nil, fmt.Errorf("cannot get definition from spec field of GrafanaDashboardDefinition: %v", err)
	}
	if !ok {
		return nil, fmt.Errorf("GrafanaDashboardDefinition has no definition inside of spec field")
	}
	return definition, nil
}

func grafanaDashboardCRDsHandler(input *go_hook.HookInput) error {
	dashboardCRDItems := input.Snapshots["grafana_dashboard_definitions"]

	if len(dashboardCRDItems) == 0 {
		return nil
	}

	dashboardPanels := make(map[string][]gjson.Result)

	for _, dashboardCRDItem := range dashboardCRDItems {
		dashboard := gjson.Parse(dashboardCRDItem.(string))
		dashboardTitle := dashboard.Get("title").String()
		dashboardRows := dashboard.Get("rows").Array()
		for _, dashboardRow := range dashboardRows {
			rowPanels := dashboardRow.Get("panels").Array()
			dashboardPanels[dashboardTitle] = append(dashboardPanels[dashboardTitle], rowPanels...)
		}
		panels := dashboard.Get("panels").Array()
		dashboardPanels[dashboardTitle] = append(dashboardPanels[dashboardTitle], panels...)
	}

	for dashboard := range dashboardPanels {
		for _, panel := range dashboardPanels[dashboard] {
			panelTitle := panel.Get("title").String()
			intervals := evaluateDeprecatedIntervals(panel)
			for _, interval := range intervals {
				input.MetricsCollector.Set("d8_grafana_dashboards_deprecated_intervals",
					1, map[string]string{
						"dashboard": sanitizeLabelName(dashboard),
						"panel":     sanitizeLabelName(panelTitle),
						"interval":  interval,
					},
				)
			}
			alert := panel.Get("alert")
			if alert.Exists() {
				alertName := alert.Get("name").String()
				input.MetricsCollector.Set("d8_grafana_dashboards_deprecated_alerts",
					1, map[string]string{
						"dashboard": sanitizeLabelName(dashboard),
						"panel":     sanitizeLabelName(panelTitle),
						"alert":     sanitizeLabelName(alertName),
					},
				)
			}
			panelType := panel.Get("type").String()
			if !isStablePanelType(panelType) {
				input.MetricsCollector.Set("d8_grafana_dashboards_deprecated_plugins",
					1, map[string]string{
						"dashboard": sanitizeLabelName(dashboard),
						"panel":     sanitizeLabelName(panelTitle),
						"plugin":    panelType,
					},
				)
			}
		}
	}

	return nil
}

var (
	deprecatedIntervals = []string{
		"interval_rv",
		"interval_sx3",
		"interval_sx4",
	}
)

func evaluateDeprecatedIntervals(panel gjson.Result) []string {
	targets := panel.Get("targets").Array()
	intervals := make([]string, 0)
	for _, target := range targets {
		expr := target.Get("expr").String()
		if deprecatedInterval, ok := evaluateDeprecatedInterval(expr); ok {
			intervals = append(intervals, deprecatedInterval)
		}
	}
	return intervals
}

func evaluateDeprecatedInterval(expression string) (string, bool) {
	for _, deprecatedInterval := range deprecatedIntervals {
		if strings.Contains(expression, deprecatedInterval) {
			return deprecatedInterval, true
		}
	}
	return "", false
}

var stablePanelTypes = []string{
	"row", // row is not a plugin type, but panel type also
	"alertGroups",
	"alertlist",
	"annolist",
	"barchart",
	"bargauge",
	"candlestick",
	"canvas",
	"dashlist",
	"datagrid",
	"debug",
	"flamegraph",
	"gauge",
	"geomap",
	"gettingstarted",
	"graph",
	"heatmap",
	"histogram",
	"live",
	"logs",
	"news",
	"nodeGraph",
	"piechart",
	"stat",
	"state_timeline",
	"status_history",
	"table",
	"table_old",
	"text",
	"timeseries",
	"traces",
	"trend",
	"welcome",
	"xychart",
}

func isStablePanelType(panelType string) bool {
	for _, stablePanelType := range stablePanelTypes {
		if stablePanelType == panelType {
			return true
		}
	}
	return false
}

func sanitizeLabelName(s string) string {
	if len(s) == 0 {
		return s
	}

	s = strings.Map(sanitizeRune, s)
	if unicode.IsDigit(rune(s[0])) {
		s = "key_" + s
	}
	if s[0] == '_' {
		s = "key" + s
	}
	return strings.ToLower(s)
}

func sanitizeRune(r rune) rune {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return r
	}
	return '_'
}
