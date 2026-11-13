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

package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
)

// Guards the built-in profiles of the module templates against values that the
// agent rejects.
func TestShippedProfilesAreValid(t *testing.T) {
	files, err := filepath.Glob("../../../../../../templates/fencing-agent/sla-profiles/*.yaml")
	if err != nil || len(files) == 0 {
		// The templates are in a repository checkout, not in every build sandbox.
		t.Skipf("shipped profiles are not reachable: %v", err)
	}

	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}

		// Drop the Helm lines: what stays is the YAML of the object.
		var lines []string

		for _, line := range strings.Split(string(raw), "\n") {
			if !strings.Contains(line, "{{") {
				lines = append(lines, line)
			}
		}

		var p v1alpha1.FencingSLAProfile
		if err := yaml.Unmarshal([]byte(strings.Join(lines, "\n")), &p); err != nil {
			t.Fatalf("%s: %v", file, err)
		}

		if err := Validate(&p); err != nil {
			t.Errorf("%s: the agent rejects the shipped profile: %v", file, err)
		}
	}
}
