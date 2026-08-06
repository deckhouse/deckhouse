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

package immutable

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

const (
	// masterNodeGroupName is the NodeGroup the first control-plane node belongs to.
	masterNodeGroupName = "master"

	// nodeGroupKind and systemTypeImmutable mirror the node-manager API.
	// node-controller is a separate Go module, so the two constants are
	// repeated here instead of imported.
	nodeGroupKind       = "NodeGroup"
	systemTypeImmutable = "Immutable"
)

// masterNodeGroupHints identify a document that failed to parse as the master
// NodeGroup, so a templated one is reported instead of silently ignored.
var masterNodeGroupHints = []*regexp.Regexp{
	regexp.MustCompile(`(?m)^\s*kind:\s*["']?NodeGroup["']?\s*$`),
	regexp.MustCompile(`(?m)^\s*name:\s*["']?master["']?\s*$`),
}

// IsImmutableMaster reports whether the master NodeGroup in the resources
// section asks for an immutable system.
//
// It runs before the resources are templated and fully parsed (bootstrap needs
// the answer while building the master's cloud-init), so a document that does
// not parse is skipped rather than rejected — the resources section may hold
// anything. The one exception is the master NodeGroup itself: dhctl has to read
// it as plain YAML this early, so a templated one is an error the user must see.
//
// Pure; the context is here for the package's uniform exported signature.
func IsImmutableMaster(_ context.Context, resourcesYAML string) (bool, error) {
	for _, doc := range input.YAMLSplitRegexp.Split(strings.TrimSpace(resourcesYAML), -1) {
		if strings.TrimSpace(doc) == "" {
			continue
		}

		var parsed struct {
			Kind     string `json:"kind"`
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Spec struct {
				SystemType string `json:"systemType"`
			} `json:"spec"`
		}

		if err := yaml.Unmarshal([]byte(doc), &parsed); err != nil {
			if !looksLikeMasterNodeGroup(doc) {
				continue
			}
			return false, fmt.Errorf("parse master NodeGroup from the resources section (templating it is not supported: dhctl reads it before rendering): %w", err)
		}

		if parsed.Kind != nodeGroupKind || parsed.Metadata.Name != masterNodeGroupName {
			continue
		}

		return parsed.Spec.SystemType == systemTypeImmutable, nil
	}

	return false, nil
}

func looksLikeMasterNodeGroup(doc string) bool {
	for _, hint := range masterNodeGroupHints {
		if !hint.MatchString(doc) {
			return false
		}
	}
	return true
}
