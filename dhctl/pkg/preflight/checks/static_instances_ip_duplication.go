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

	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

type StaticInstancesIPDuplicationCheck struct {
	MetaConfig *config.MetaConfig
}

type staticInstanceDocument struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Address string `yaml:"address"`
	} `yaml:"spec"`
}

const StaticInstancesIPDuplicationCheckName preflight.CheckName = "static-instances-ip-duplication"

func (StaticInstancesIPDuplicationCheck) Description() string {
	return "static instances have unique addresses"
}

func (StaticInstancesIPDuplicationCheck) Phase() preflight.Phase {
	return preflight.PhasePreInfra
}

func (StaticInstancesIPDuplicationCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.RetryPolicy{Attempts: 1}
}

func (c StaticInstancesIPDuplicationCheck) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, preflight.DefaultPreflightCheckTimeout)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return err
	}

	if c.MetaConfig == nil || c.MetaConfig.ResourcesYAML == "" {
		return nil
	}

	documents := input.YAMLSplitRegexp.Split(c.MetaConfig.ResourcesYAML, -1)
	instances := make(map[string]string)

	for _, doc := range documents {
		var result map[string]interface{}
		err := yaml.Unmarshal([]byte(doc), &result)
		if err != nil {
			return fmt.Errorf("cannot unmarshal YAML: %v", err)
		}

		if result["kind"] == "StaticInstance" {
			meta := result["metadata"].(map[string]interface{})
			name := meta["name"].(string)

			spec := result["spec"].(map[string]interface{})
			address := spec["address"].(string)

			instName, ok := instances[address]
			if ok {
				return fmt.Errorf("Duplicate address for %s: %s and %s\n", address, instName, name)
			} else {
				instances[address] = name
			}
		}
	}

	return nil
}

func StaticInstancesIPDuplication(meta *config.MetaConfig) preflight.Check {
	check := StaticInstancesIPDuplicationCheck{MetaConfig: meta}
	return preflight.Check{
		Name:        StaticInstancesIPDuplicationCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}
