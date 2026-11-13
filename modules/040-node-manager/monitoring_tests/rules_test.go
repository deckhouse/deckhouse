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

// Package monitoring_tests runs promtool on the test files of the alerts of the module.
package monitoring_tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const rulesDir = "../monitoring/prometheus-rules"

// The rule files of the module are lists of groups: the chart adds the top-level key `groups`.
func TestPromtoolRules(t *testing.T) {
	promtool, err := exec.LookPath("promtool")
	if err != nil {
		// make bin/promtool installs it.
		promtool = "../../../bin/promtool"
		if _, err := os.Stat(promtool); err != nil {
			t.Skip("promtool is not installed")
		}
	}

	dir := t.TempDir()
	for _, rules := range []string{"fencing.yaml", "fencing-controller.yaml", "node-group-deprecate.yaml"} {
		body, err := os.ReadFile(filepath.Join(rulesDir, rules))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, rules), append([]byte("groups:\n"), body...), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	tests, err := filepath.Glob("*.test.yaml")
	if err != nil || len(tests) == 0 {
		t.Fatalf("no test files: %v", err)
	}
	for _, test := range tests {
		body, err := os.ReadFile(test)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, test), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	command := exec.Command(promtool, append([]string{"test", "rules"}, tests...)...)
	command.Dir = dir
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("promtool: %v\n%s", err, out)
	}
}
