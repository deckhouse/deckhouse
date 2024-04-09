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
		strings.HasSuffix(fileName, ".json")
}

func validateGrafanaDashboardFile(fileName string, fileContent []byte) *Messages {
	msgs := NewMessages()

	dashboard := gjson.ParseBytes(fileContent)
	dashboardPanels := make([]gjson.Result, 0)
	dashboardRows := dashboard.Get("rows").Array()

	for _, dashboardRow := range dashboardRows {
		rowPanels := dashboardRow.Get("panels").Array()
		dashboardPanels = append(dashboardPanels, rowPanels...)
	}
	panels := dashboard.Get("panels").Array()
	dashboardPanels = append(dashboardPanels, panels...)

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
		legacyDatasourceUIDs, hardcodedDatasourceUIDs := evaluateDeprecatedDatasourceUIDs(panel)
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
	}
	return msgs
}

func evaluateDeprecatedDatasourceUIDs(panel gjson.Result) (legacyUIDs, hardcodedUIDs []string) {
	targets := panel.Get("targets").Array()
	legacyUIDs = make([]string, 0)
	hardcodedUIDs = make([]string, 0)
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
		}
	}
	return hardcodedUIDs, legacyUIDs
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
		"flant-statusmap-panel": "status-history",
	}
)

func evaluateDeprecatedPanelType(panelType string) (replaceWith string, isDeprecated bool) {
	replaceWith, isDeprecated = deprecatedPanelTypes[panelType]
	return replaceWith, isDeprecated
}
