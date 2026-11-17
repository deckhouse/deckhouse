/*
Copyright 2026 Flant JSC

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

package monitoring_tests

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"gopkg.in/yaml.v3"
)

const alertingRules = "../monitoring/prometheus-rules/alerting.yaml"

func TestPromtoolRules(t *testing.T) {
	promtool, err := exec.LookPath("promtool")
	if err != nil {
		promtool, err = filepath.Abs("../../../bin/promtool")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(promtool); err != nil {
			t.Skip("promtool is not installed; run make bin/promtool")
		}
	}

	body, err := os.ReadFile(alertingRules)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	// Module rules contain groups without the top-level key added by the chart.
	if err := os.WriteFile(filepath.Join(dir, "alerting.yaml"), append([]byte("groups:\n"), body...), 0o600); err != nil {
		t.Fatal(err)
	}
	const testFile = "application-namespace.test.yaml"
	body, err = os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, testFile), body, 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(promtool, "test", "rules", testFile)
	command.Dir = dir
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("promtool: %v\n%s", err, out)
	}
}

func TestApplicationAlertAnnotations(t *testing.T) {
	body, err := os.ReadFile(alertingRules)
	if err != nil {
		t.Fatal(err)
	}
	var groups []struct {
		Rules []struct {
			Alert       string            `yaml:"alert"`
			Annotations map[string]string `yaml:"annotations"`
		} `yaml:"rules"`
	}
	if err := yaml.Unmarshal(body, &groups); err != nil {
		t.Fatal(err)
	}

	commands := map[string]string{
		"ApplicationIsInMaintenanceMode":            "d8 k patch application shop -n usage-marketplace",
		"ApplicationHorizontalPodAutoscalerMissing": "d8 k get application shop -n usage-marketplace",
		"ApplicationVerticalPodAutoscalerMissing":   "d8 k get application shop -n usage-marketplace",
		"ApplicationHorizontalPodAutoscalerPinned":  "d8 k get application shop -n usage-marketplace",
		"ApplicationVerticalPodAutoscalerPinned":    "d8 k get application shop -n usage-marketplace",
	}
	labels := map[string]string{
		"namespace":             "d8-system",
		"application_namespace": "usage-marketplace",
		"name":                  "shop",
		"kind":                  "Deployment",
		"workload":              "d8a-shop-web",
		"container":             "app",
	}
	for _, group := range groups {
		for _, rule := range group.Rules {
			command, ok := commands[rule.Alert]
			if !ok {
				continue
			}
			delete(commands, rule.Alert)
			t.Run(rule.Alert, func(t *testing.T) {
				for _, name := range []string{"summary", "description"} {
					t.Run(name, func(t *testing.T) {
						tmpl, err := template.New(name).Option("missingkey=error").Parse("{{ $labels := . }}" + rule.Annotations[name])
						if err != nil {
							t.Fatal(err)
						}
						var rendered bytes.Buffer
						if err := tmpl.Execute(&rendered, labels); err != nil {
							t.Fatal(err)
						}
						text := rendered.String()
						if !strings.Contains(text, "namespace `usage-marketplace`") || strings.Contains(text, "d8-system") {
							t.Errorf("annotation must identify the application namespace: %s", text)
						}
						if name == "description" && !strings.Contains(text, command) {
							t.Errorf("description must contain %q: %s", command, text)
						}
					})
				}
			})
		}
	}
	if len(commands) > 0 {
		t.Errorf("application alert rules are missing: %v", commands)
	}
}
