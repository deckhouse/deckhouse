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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/300-prometheus/hooks/internal/simplejson"
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
		return nil, fmt.Errorf("cannot definition from definition of GrafanaDashboardDefinition: %v", err)
	}
	if !ok {
		return nil, fmt.Errorf("has no definition field inside definition of GrafanaDashboardDefinition")
	}
	return definition, nil
}

func grafanaDashboardCRDsHandler(input *go_hook.HookInput) error {
	dashboardCRDItems := input.Snapshots["grafana_dashboard_definitions"]

	if len(dashboardCRDItems) == 0 {
		return nil
	}

	dashboardPanels := make(map[string][]*simplejson.Json)

	for _, dashboardCRDItem := range dashboardCRDItems {
		dashboardCRD := dashboardCRDItem.(string)
		dashboard, err := simplejson.NewJson([]byte(dashboardCRD))
		if err != nil {
			return err
		}
		dashboardTitle := getTitle(dashboard)
		rows := getRows(dashboard)
		for _, row := range rows {
			rowPanels := getPanels(row)
			dashboardPanels[dashboardTitle] = append(dashboardPanels[dashboardTitle], rowPanels...)
		}
		panels := getPanels(dashboard)
		dashboardPanels[dashboardTitle] = append(dashboardPanels[dashboardTitle], panels...)
	}

	for dashboard := range dashboardPanels {
		for _, panel := range dashboardPanels[dashboard] {
			panelTitle := getTitle(panel)
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
			alerts := evaluateDeprecatedAlerts(panel)
			for _, alert := range alerts {
				input.MetricsCollector.Set("d8_grafana_dashboards_deprecated_alerts",
					1, map[string]string{
						"dashboard": sanitizeLabelName(dashboard),
						"panel":     sanitizeLabelName(panelTitle),
						"alert":     sanitizeLabelName(alert),
					},
				)
			}
			panelType := getType(panel)
			if isUnstablePanelType(panelType) {
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

func getTitle(data *simplejson.Json) string {
	return getStringFieldValue(data, "title")
}

func getName(data *simplejson.Json) string {
	return getStringFieldValue(data, "name")
}

func getType(data *simplejson.Json) string {
	return getStringFieldValue(data, "type")
}

func getStringFieldValue(data *simplejson.Json, fieldName string) string {
	value, hasValue := data.CheckGet(fieldName)
	if !hasValue {
		return ""
	}
	valueData, err := value.String()
	if err != nil {
		return ""
	}
	return valueData
}

func getPanels(data *simplejson.Json) []*simplejson.Json {
	panels, hasPanels := data.CheckGet("panels")
	if !hasPanels {
		return nil
	}
	panelsData, err := panels.Array()
	if err != nil {
		return nil
	}
	list := make([]*simplejson.Json, 0, len(panelsData))
	for _, panelsDataItem := range panelsData {
		list = append(list, simplejson.NewFromAny(panelsDataItem))
	}
	return list
}

func getRows(data *simplejson.Json) []*simplejson.Json {
	rows, hasRows := data.CheckGet("rows")
	if !hasRows {
		return nil
	}
	rowsData, err := rows.Array()
	if err != nil {
		return nil
	}
	list := make([]*simplejson.Json, 0, len(rowsData))
	for _, rowsDataItem := range rowsData {
		list = append(list, simplejson.NewFromAny(rowsDataItem))
	}
	return list
}

var (
	deprecatedIntervals = []string{
		"interval_rv",
		"interval_sx3",
		"interval_sx4",
	}
)

func evaluateDeprecatedIntervals(panel *simplejson.Json) []string {
	targets, err := panel.Get("targets").Array()
	if err != nil {
		return nil
	}
	intervals := make([]string, 0)
	for _, target := range targets {
		targetData := simplejson.NewFromAny(target)
		expr := targetData.Get("expr")
		exprData, err := expr.String()
		if err != nil {
			return nil
		}
		if deprecatedInterval, ok := evaluateDeprecatedInterval(exprData); ok {
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

func evaluateDeprecatedAlerts(panel *simplejson.Json) []string {
	alertNames := make([]string, 0)
	alert, hasAlert := panel.CheckGet("alert")
	if hasAlert {
		name := getName(alert)
		alertNames = append(alertNames, name)
	}
	return alertNames
}

var stablePanelTypes = []string{
	// row is not a plugin type, but panel type also
	"row",
	// flant-statusmap-panel works in Grafana 10.x
	"flant-statusmap-panel",
	// core plugin types
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

func isUnstablePanelType(panelType string) bool {
	for _, stablePanelType := range stablePanelTypes {
		if stablePanelType == panelType {
			return false
		}
	}
	return true
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
