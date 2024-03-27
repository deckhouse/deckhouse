/*
Copyright 2024 Flant JSC

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
	"regexp"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/tidwall/gjson"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/prometheus/deprecate_outdated_grafana_dashboard_crd",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "helm_releases",
			Crontab: "0 * * * *", // every hour
		},
	},
}, dependency.WithExternalDependencies(handleGrafanaDashboardCRDs))

func handleGrafanaDashboardCRDs(input *go_hook.HookInput, dc dependency.Container) error {
	client, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	ctx := context.Background()
	dashboardCRDItems, err := listDashboardCRDs(ctx, client.Dynamic())
	if err != nil {
		return err
	}

	if len(dashboardCRDItems) == 0 {
		return nil
	}

	dashboardPanels := make(map[string][]gjson.Result)

	for _, dashboardCRDItem := range dashboardCRDItems {
		dashboard := gjson.Parse(dashboardCRDItem.Spec.Definition)
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
				input.MetricsCollector.Set("d8_grafana_dashboards_deprecated_interval",
					1, map[string]string{
						"dashboard": sanitizeLabelName(dashboard),
						"panel":     sanitizeLabelName(panelTitle),
						"interval":  interval,
					},
				)
			}
			alertRule := panel.Get("alert")
			if alertRule.Exists() {
				alertRuleName := alertRule.Get("name").String()
				input.MetricsCollector.Set("d8_grafana_dashboards_deprecated_alert_rule",
					1, map[string]string{
						"dashboard":  sanitizeLabelName(dashboard),
						"panel":      sanitizeLabelName(panelTitle),
						"alert_rule": sanitizeLabelName(alertRuleName),
					},
				)
			}
			panelType := panel.Get("type").String()
			if evaluateDeprecatedPlugin(panelType) {
				input.MetricsCollector.Set("d8_grafana_dashboards_deprecated_plugin",
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

const (
	dashboardResourceName    = "grafanadashboarddefinitions"
	dashboardResourceGroup   = "deckhouse.io"
	dashboardResourceVersion = "v1"
)

var (
	dashboardCRDSchema = schema.GroupVersionResource{
		Resource: dashboardResourceName,
		Group:    dashboardResourceGroup,
		Version:  dashboardResourceVersion,
	}
)

// DashboardCRD is a model of Dashboard CRD stored in k8s
type DashboardCRD struct {
	Spec DashboardCRDSpec `json:"spec"`
}

// DashboardCRDSpec contains Dashboard JSON and folder name
type DashboardCRDSpec struct {
	Definition string `json:"definition"`
	Folder     string `json:"folder"`
}

func listDashboardCRDs(ctx context.Context, dynamicClient dynamic.Interface) ([]*DashboardCRD, error) {
	unstructuredList, err := dynamicClient.Resource(dashboardCRDSchema).List(ctx, v1.ListOptions{})
	if err != nil {
		return nil, err
	}
	list := make([]*DashboardCRD, 0, len(unstructuredList.Items))
	for _, unstructuredListItem := range unstructuredList.Items {
		var dashboardCRD DashboardCRD
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(
			unstructuredListItem.UnstructuredContent(), &dashboardCRD,
		)
		if err != nil {
			return nil, err
		}
		list = append(list, &dashboardCRD)
	}
	return list, nil
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

var (
	deprecatedPlugins = []string{
		"flant-statusmap-panel",
	}
)

func evaluateDeprecatedPlugin(plugin string) bool {
	for _, deprecatedPlugin := range deprecatedPlugins {
		if deprecatedPlugin == plugin {
			return true
		}
	}
	return false
}

var prometheusLabelNameRegexp = regexp.MustCompile(`[^a-zA-Z0-9_]`)

func sanitizeLabelName(s string) string {
	return strings.ToLower(prometheusLabelNameRegexp.ReplaceAllString(s, "_"))
}
