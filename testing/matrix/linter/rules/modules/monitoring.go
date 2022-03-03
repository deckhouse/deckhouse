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

package modules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/errors"
)

func dirExists(moduleName, modulePath string, path ...string) (bool, errors.LintRuleError) {
	searchPath := filepath.Join(append([]string{modulePath}, path...)...)
	info, err := os.Stat(searchPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, errors.EmptyRuleError
		}
		return false, errors.NewLintRuleError(
			"MODULE060",
			moduleLabel(moduleName),
			path,
			err.Error(),
		)
	}
	return info.IsDir(), errors.EmptyRuleError
}

func monitoringModuleRule(moduleName, modulePath, moduleNamespace string) errors.LintRuleError {
	switch moduleName {
	// These modules deploy common rules and dashboards to the cluster according to their configurations.
	// That's why they have custom monitoring templates.
	case "340-extended-monitoring", "340-monitoring-applications", "030-cloud-provider-yandex":
		return errors.EmptyRuleError
	}

	folderEx, lerr := dirExists(moduleName, modulePath, "monitoring")
	if !lerr.IsEmpty() {
		return lerr
	}

	if !folderEx {
		return errors.EmptyRuleError
	}

	rulesEx, lerr := dirExists(moduleName, modulePath, "monitoring", "prometheus-rules")
	if !lerr.IsEmpty() {
		return lerr
	}

	dashboardsEx, lerr := dirExists(moduleName, modulePath, "monitoring", "grafana-dashboards")
	if !lerr.IsEmpty() {
		return lerr
	}

	searchingFilePath := filepath.Join(modulePath, "templates", "monitoring.yaml")
	info, _ := os.Stat(searchingFilePath)
	if info == nil {
		return errors.NewLintRuleError(
			"MODULE060",
			moduleLabel(moduleName),
			searchingFilePath,
			"Module with the 'monitoring' folder should have the 'templates/monitoring.yaml' file",
		)
	}

	content, err := os.ReadFile(searchingFilePath)
	if err != nil {
		return errors.NewLintRuleError(
			"MODULE060",
			moduleLabel(moduleName),
			searchingFilePath,
			err.Error(),
		)
	}

	desiredContentBuilder := strings.Builder{}
	if dashboardsEx {
		desiredContentBuilder.WriteString("{{- include \"helm_lib_grafana_dashboard_definitions\" . }}\n")
	}

	if rulesEx {
		desiredContentBuilder.WriteString(
			"{{- include \"helm_lib_prometheus_rules\" (list . %q) }}\n",
		)
	}

	var res bool
	for _, namespace := range []string{moduleNamespace, "d8-system", "d8-monitoring"} {
		desiredContent := fmt.Sprintf(desiredContentBuilder.String(), namespace)
		res = res || desiredContent == string(content)
	}

	if !res {
		return errors.NewLintRuleError(
			"MODULE060",
			moduleLabel(moduleName),
			searchingFilePath,
			"The content of the 'templates/monitoring.yaml' should be equal to:\n%s\nGot:\n%s",
			fmt.Sprintf(desiredContentBuilder.String(), "YOUR NAMESAPCE TO DEPLOY RULES: d8-monitoring, d8-system or module namespaces"),
			string(content),
		)
	}

	return errors.EmptyRuleError
}

func compareContent(content, namespace string, rules, dashboards bool) bool {
	desiredContentBuilder := strings.Builder{}
	if dashboards {
		desiredContentBuilder.WriteString("{{- include \"helm_lib_grafana_dashboard_definitions\" . }}\n")
	}

	if rules {
		desiredContentBuilder.WriteString(
			"{{- include \"helm_lib_prometheus_rules\" (list . %q) }}\n",
		)
	}

	return content == desiredContentBuilder.String()
}
