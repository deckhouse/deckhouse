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

package fsprovider

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/vmchange"
)

const planRulesFilename = "plan_rules.yml"

type planRulesEntry struct {
	VMChange *vmchange.Rule `json:"vmChange,omitempty"`
}

// loadPlanRules reads plan_rules.yml sitting next to the infrastructure
// versions file. Missing file is not an error — providers without a rule fall
// back to VMResourceType matching. Returns a nil map when the file is absent.
func loadPlanRules(infraVersionsFile string) (map[string]*vmchange.Rule, error) {
	path := filepath.Join(filepath.Dir(infraVersionsFile), planRulesFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read plan rules file %s: %w", path, err)
	}

	raw := map[string]planRulesEntry{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal plan rules file %s: %w", path, err)
	}

	out := make(map[string]*vmchange.Rule, len(raw))
	for providerKey, entry := range raw {
		if entry.VMChange != nil {
			out[providerKey] = entry.VMChange
		}
	}
	return out, nil
}
