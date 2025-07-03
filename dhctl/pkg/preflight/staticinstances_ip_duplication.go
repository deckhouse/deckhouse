// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package preflight

import (
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"gopkg.in/yaml.v3"
)

func (pc *Checker) CheckStaticInstancesIPDuplication(_ context.Context) error {
	documents := input.YAMLSplitRegexp.Split(pc.metaConfig.ResourcesYAML, -1)
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
