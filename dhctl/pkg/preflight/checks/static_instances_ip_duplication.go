// Copyright 2026 Flant JSC
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

package checks

import (
	"context"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

type StaticInstancesIPDuplicationCheck struct {
	MetaConfig *config.MetaConfig
}

const StaticInstancesIPDuplicationCheckName preflight.CheckName = "static-instances-ip-duplication"

func (StaticInstancesIPDuplicationCheck) Description() string {
	return "StaticInstances have unique addresses"
}

func (StaticInstancesIPDuplicationCheck) Phase() preflight.Phase {
	return preflight.PhasePreInfra
}

func (StaticInstancesIPDuplicationCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

func (c StaticInstancesIPDuplicationCheck) Run(_ context.Context) (string, error) {
	if c.MetaConfig == nil || c.MetaConfig.ResourcesYAML == "" {
		return "", preflight.NotApplicable("the --config file declares no resources")
	}

	documents := input.YAMLSplitRegexp.Split(c.MetaConfig.ResourcesYAML, -1)

	// address -> the StaticInstance that claimed it first.
	instances := make(map[string]string)
	var duplicates []string

	for i, doc := range documents {
		var result map[string]any
		if err := yaml.Unmarshal([]byte(doc), &result); err != nil {
			return "", preflight.Permanent(&preflight.Failure{
				Checked:  fmt.Sprintf("resources document #%d in the --config file", i+1),
				Observed: err.Error(),
				Expected: "a YAML document",
				Fix:      "correct the document, or remove it from the --config file",
			})
		}
		if result["kind"] != "StaticInstance" {
			continue
		}

		// Read rather than assert: these are operator-written documents, and a StaticInstance
		// with a missing spec or a numeric name used to panic the check — a Go stack trace in
		// place of the sentence naming the document.
		name := nestedString(result, "metadata", "name")
		address := nestedString(result, "spec", "address")
		if name == "" {
			name = fmt.Sprintf("document #%d", i+1)
		}
		if address == "" {
			return "", preflight.Permanent(&preflight.Failure{
				Checked:  fmt.Sprintf("StaticInstance %q in the --config file", name),
				Observed: "spec.address is missing or is not a string",
				Expected: "the address Deckhouse will reach the machine at",
				Fix:      fmt.Sprintf("set spec.address on StaticInstance %q", name),
			})
		}

		if first, taken := instances[address]; taken {
			// All of them, not the first: with several duplicates the operator would otherwise
			// fix one pair per run.
			duplicates = append(duplicates, fmt.Sprintf("%s: %s and %s", address, first, name))
			continue
		}
		instances[address] = name
	}

	if len(duplicates) > 0 {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  "the spec.address of every StaticInstance in the --config file",
			Observed: "- " + strings.Join(duplicates, "\n- "),
			Expected: "one StaticInstance per address",
			Fix:      "give each machine its own StaticInstance, or remove the duplicates",
		})
	}

	if len(instances) == 0 {
		return "", preflight.NotApplicable("the --config file declares no StaticInstance")
	}
	return fmt.Sprintf("%d StaticInstances have distinct addresses", len(instances)), nil
}

// nestedString reads a string at a path, and returns "" for anything that is not one.
func nestedString(object map[string]any, path ...string) string {
	var current any = object
	for _, key := range path {
		asMap, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = asMap[key]
	}
	value, _ := current.(string)
	return value
}

func StaticInstancesIPDuplication(meta *config.MetaConfig) preflight.Check {
	check := StaticInstancesIPDuplicationCheck{MetaConfig: meta}
	return preflight.Check{
		Name:        StaticInstancesIPDuplicationCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Cacheable:   true,
		Run:         check.Run,
	}
}
