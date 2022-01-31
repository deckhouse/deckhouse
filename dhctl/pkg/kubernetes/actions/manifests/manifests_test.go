// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package manifests

import (
	"fmt"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

func Test_struct_vs_unmarshal(t *testing.T) {
	params := DeckhouseDeploymentParams{
		Registry:         "registry.example.com/deckhouse:master",
		LogLevel:         "debug",
		Bundle:           "default",
		IsSecureRegistry: true,
	}

	depl1 := DeckhouseDeployment(params)

	depl2 := DeckhouseDeployment(params)

	depl1Yaml, err := yaml.Marshal(depl1)
	if err != nil {
		t.Errorf("deployment from struct unmarshal: %v", err)
	}
	depl2Yaml, err := yaml.Marshal(depl2)
	if err != nil {
		t.Errorf("deployment from yaml unmarshal: %v", err)
	}

	depl1Lines := strings.Split(string(depl1Yaml), "\n")
	depl2Lines := strings.Split(string(depl2Yaml), "\n")

	if len(depl1Lines) != len(depl2Lines) {
		t.Logf("depl1 lines: %d, depl2 lines: %d", len(depl1Lines), len(depl2Lines))
	}

	i := 0
	diff := 0
	for {
		l1 := ""
		l2 := ""

		if i < len(depl1Lines) {
			l1 = depl1Lines[i]
		}

		if i < len(depl2Lines) {
			l2 = depl2Lines[i]
		}

		mark := " "
		if l1 != l2 {
			mark = "!"
			diff++
		}

		fmt.Printf("%s %-35s %-35s\n", mark, l1, l2)
		i++
		if i >= len(depl1Lines) && i >= len(depl2Lines) {
			break
		}
	}

	fmt.Printf("%d lines are differ\n", diff)
}
