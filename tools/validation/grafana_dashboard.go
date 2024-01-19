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
		datasourceUIDs := evaluateHardcodedDatasourceUIDs(panel)
		for _, datasourceUID := range datasourceUIDs {
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

func evaluateHardcodedDatasourceUIDs(panel gjson.Result) []string {
	targets := panel.Get("targets").Array()
	datasourceUIDs := make([]string, 0)
	for _, target := range targets {
		datasource := target.Get("datasource")
		if datasource.Exists() {
			uid := datasource.Get("uid").String()
			if !strings.HasPrefix(uid, "$") {
				datasourceUIDs = append(datasourceUIDs, uid)
			}
		}
	}
	return datasourceUIDs
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
