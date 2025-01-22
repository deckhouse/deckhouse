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

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/tidwall/gjson"
)

func RunGrafanaDashboardValidation(info *DiffInfo) (exitCode int) {
	fmt.Println("Run 'grafana dashboards' validation...")

	if len(info.Files) == 0 {
		fmt.Println("Nothing to validate, diff is empty.")
		return 0
	}

	exitCode = 0
	msgs := NewMessages()
	for _, fileInfo := range info.Files {
		if !fileInfo.HasContent() {
			continue
		}

		fileName := fileInfo.NewFileName
		if !isGrafanaDashboard(fileName) {
			continue
		}

		fileContent, err := os.ReadFile(fileName)
		if err != nil {
			msgs.Add(
				NewError(
					fileName,
					"failed to open file",
					err.Error(),
				))
			continue
		}

		msgs.Join(validateGrafanaDashboardFile(fileName, fileContent))
	}

	msgs.PrintReport()
	if msgs.CountErrors() > 0 {
		exitCode = 1
	}

	return exitCode
}

func isGrafanaDashboard(fileName string) bool {
	fileName = strings.ToLower(fileName)
	return strings.Contains(fileName, "grafana-dashboards") &&
		(strings.HasSuffix(fileName, ".json") || strings.HasSuffix(fileName, ".tpl"))
}

func validateGrafanaDashboardFile(fileName string, fileContent []byte) *Messages {
	fmt.Printf("Validating %s grafana dashboard definition\n", fileName)
	msgs := NewMessages()

	dashboard := gjson.ParseBytes(fileContent)
	dashboardPanels := extractDashboardPanels(dashboard)
	dashboardTemplates := extractDashboardTemplates(dashboard)

	for _, panel := range dashboardPanels {
		panelTitle := panel.Get("title").String()
		panelType := panel.Get("type").String()
		replaceWith, isDeprecated := evaluateDeprecatedPanelType(panelType)
		if isDeprecated {
			msgs.Add(
				NewError(
					fileName,
					"deprecated panel type",
					fmt.Sprintf("Panel %s is of deprecated type: '%s', consider using '%s'",
						panelTitle, panelType, replaceWith),
				),
			)
		}
		intervals := evaluateDeprecatedIntervals(panel)
		for _, interval := range intervals {
			msgs.Add(
				NewError(
					fileName,
					"deprecated interval",
					fmt.Sprintf("Panel %s contains deprecated interval: '%s', consider using '$__rate_interval'",
						panelTitle, interval),
				),
			)
		}
		alertRule := panel.Get("alert")
		if alertRule.Exists() {
			alertRuleName := alertRule.Get("name").String()
			msgs.Add(
				NewError(
					fileName,
					"legacy alert rule",
					fmt.Sprintf("Panel %s contains legacy alert rule: '%s', consider using external alertmanager",
						panelTitle, alertRuleName),
				),
			)
		}
		legacyDatasourceUIDs, hardcodedDatasourceUIDs, nonRecommendedPrometheusDatasourceUIDs := evaluateDeprecatedDatasourceUIDs(panel)
		for _, datasourceUID := range legacyDatasourceUIDs {
			msgs.Add(
				NewError(
					fileName,
					"legacy datasource uid",
					fmt.Sprintf("Panel %s contains legacy datasource uid: '%s', consider resaving dashboard using newer version of Grafana",
						panelTitle, datasourceUID),
				),
			)
		}
		for _, datasourceUID := range hardcodedDatasourceUIDs {
			msgs.Add(
				NewError(
					fileName,
					"hardcoded datasource uid",
					fmt.Sprintf("Panel %s contains hardcoded datasource uid: '%s', consider using grafana variable of type 'Datasource'",
						panelTitle, datasourceUID),
				),
			)
		}
		for _, datasourceUID := range nonRecommendedPrometheusDatasourceUIDs {
			msgs.Add(
				NewError(
					fileName,
					"non-recommended prometheus datasource uid",
					fmt.Sprintf("Panel %s datasource must be one of: %s instead of '%s'",
						panelTitle, prometheusDatasourceRecommendedUIDsMessageString, datasourceUID),
				),
			)
		}
	}

	var hasPrometheusDatasourceVariable bool
	for _, dashboardTemplate := range dashboardTemplates {
		if evaluatePrometheusDatasourceTemplateVariable(dashboardTemplate) {
			hasPrometheusDatasourceVariable = true
		}
		if queryVariable, ok := evaluateNonRecommendedPrometheusDatasourceQueryTemplateVariable(dashboardTemplate); ok {
			msgs.Add(
				NewError(
					fileName,
					"non-recommended prometheus datasource query variable",
					fmt.Sprintf("Dashboard variable '%s' must use one of: %s as it's datasource", queryVariable, prometheusDatasourceRecommendedUIDsMessageString),
				),
			)
		}
	}
	if !hasPrometheusDatasourceVariable {
		msgs.Add(
			NewError(
				fileName,
				"missing prometheus datasource variable",
				fmt.Sprintf("Dashboard must contain prometheus variable with query type: '%s' and name: '%s'",
					prometheusDatasourceQuery, prometheusDatasourceRecommendedName),
			),
		)
	}
	return msgs
}

func extractDashboardPanels(dashboard gjson.Result) []gjson.Result {
	dashboardPanels := make([]gjson.Result, 0)
	dashboardRows := dashboard.Get("rows").Array()
	for _, dashboardRow := range dashboardRows {
		rowPanels := dashboardRow.Get("panels").Array()
		dashboardPanels = append(dashboardPanels, rowPanels...)
	}

	panels := dashboard.Get("panels").Array()
	for _, panel := range panels {
		panelType := panel.Get("type").String()
		if panelType == "row" {
			rowPanels := panel.Get("panels").Array()
			dashboardPanels = append(dashboardPanels, rowPanels...)
		} else {
			dashboardPanels = append(dashboardPanels, panel)
		}
	}
	return dashboardPanels
}

func extractDashboardTemplates(dashboard gjson.Result) []gjson.Result {
	dashboardTemplating := dashboard.Get("templating")
	if !dashboardTemplating.Exists() {
		return []gjson.Result{}
	}
	dashboardTemplatesList := dashboardTemplating.Get("list")
	if !dashboardTemplatesList.Exists() || !dashboardTemplatesList.IsArray() {
		return []gjson.Result{}
	}
	return dashboardTemplatesList.Array()
}

const (
	prometheusDatasourceType            = "prometheus"
	prometheusDatasourceQuery           = "prometheus"
	prometheusDatasourceRecommendedName = "ds_prometheus"
)

var (
	// both ${datasource_uid} and $datasource_uid will be parsed as $datasource_uid
	prometheusDatasourceRecommendedUIDs = []string{
		"$" + prometheusDatasourceRecommendedName,
		"${" + prometheusDatasourceRecommendedName + "}",
	}
	prometheusDatasourceRecommendedUIDsMessageString = func() string {
		res := make([]string, 0, len(prometheusDatasourceRecommendedUIDs))
		for _, prometheusDatasourceRecommendedUID := range prometheusDatasourceRecommendedUIDs {
			res = append(res, fmt.Sprintf("'%s'", prometheusDatasourceRecommendedUID))
		}
		return strings.Join(res, ", ")
	}()
)

func isRecommendedPrometheusDatasourceUID(prometheusDatasourceUID string) bool {
	for _, prometheusDatasourceRecommendedUID := range prometheusDatasourceRecommendedUIDs {
		if prometheusDatasourceRecommendedUID == prometheusDatasourceUID {
			return true
		}
	}
	return false
}

func evaluateDeprecatedDatasourceUIDs(panel gjson.Result) (
	legacyUIDs, hardcodedUIDs, nonRecommendedPrometheusUIDs []string,
) {
	targets := panel.Get("targets").Array()
	legacyUIDs = make([]string, 0)
	hardcodedUIDs = make([]string, 0)
	nonRecommendedPrometheusUIDs = make([]string, 0)
	for _, target := range targets {
		datasource := target.Get("datasource")
		if datasource.Exists() {
			var uidStr string
			uid := datasource.Get("uid")
			if uid.Exists() {
				uidStr = uid.String()
			} else {
				// some old dashboards (before Grafana 8.3) may implicitly contain uid as a string, not as parameter
				uidStr = datasource.String()
				legacyUIDs = append(legacyUIDs, uidStr)
			}
			if !strings.HasPrefix(uidStr, "$") {
				hardcodedUIDs = append(hardcodedUIDs, uidStr)
			}
			datasourceType := datasource.Get("type")
			if datasourceType.Exists() {
				datasourceTypeStr := datasourceType.String()
				if datasourceTypeStr == prometheusDatasourceType && !isRecommendedPrometheusDatasourceUID(uidStr) {
					nonRecommendedPrometheusUIDs = append(nonRecommendedPrometheusUIDs, uidStr)
				}
			}
		}
	}
	return hardcodedUIDs, legacyUIDs, nonRecommendedPrometheusUIDs
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
	deprecatedPanelTypes = map[string]string{
		"graph":                 "timeseries",
		"flant-statusmap-panel": "state-timeline",
	}
)

func evaluateDeprecatedPanelType(panelType string) (replaceWith string, isDeprecated bool) {
	replaceWith, isDeprecated = deprecatedPanelTypes[panelType]
	return replaceWith, isDeprecated
}

func evaluatePrometheusDatasourceTemplateVariable(dashboardTemplate gjson.Result) bool {
	templateType := dashboardTemplate.Get("type")
	if !templateType.Exists() {
		return false
	}
	if templateType.String() != "datasource" {
		return false
	}
	queryType := dashboardTemplate.Get("query")
	if queryType.String() != prometheusDatasourceQuery {
		return false
	}
	templateName := dashboardTemplate.Get("name")
	if templateName.String() != prometheusDatasourceRecommendedName {
		return false
	}
	return true
}

func evaluateNonRecommendedPrometheusDatasourceQueryTemplateVariable(dashboardTemplate gjson.Result) (string, bool) {
	templateType := dashboardTemplate.Get("type")
	if !templateType.Exists() {
		return "", false
	}
	if templateType.String() != "query" {
		return "", false
	}
	datasource := dashboardTemplate.Get("datasource")
	if !datasource.Exists() {
		return "", false
	}
	datasourceType := datasource.Get("type")
	if datasourceType.String() != prometheusDatasourceType {
		return "", false
	}
	datasourceUID := datasource.Get("uid")
	if isRecommendedPrometheusDatasourceUID(datasourceUID.String()) {
		return "", false
	}
	templateName := dashboardTemplate.Get("name")
	return templateName.String(), true
}
