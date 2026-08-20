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

package bootstrap

import (
	"strings"

	libdhctlyaml "github.com/deckhouse/lib-dhctl/pkg/yaml"
	yamlvalidation "github.com/deckhouse/lib-dhctl/pkg/yaml/validation"

	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
)

// splitNodeCustomizations takes the NodeConfig documents out of the resources:
// they describe machines the installer talks to, not objects to create. Left in
// place, the resources phase applies them and fails on the spec the CRD wants.
func splitNodeCustomizations(resourcesYAML string) ([]string, string) {
	var customizations, rest []string

	// The index reports group and version apart, and the payload names them together.
	group, _, _ := strings.Cut(immutable.PayloadAPIVersion, "/")

	for _, document := range libdhctlyaml.SplitYAML(resourcesYAML) {
		if strings.TrimSpace(document) == "" {
			continue
		}
		index, err := yamlvalidation.ParseIndex(strings.NewReader(document))
		// A document this fails on is not ours to judge: it goes back to the
		// resources, where the existing validation reports it.
		if err != nil || index.Kind != immutable.NodeConfigKind || index.Group() != group {
			rest = append(rest, document)
			continue
		}
		customizations = append(customizations, document)
	}

	return customizations, strings.TrimSpace(strings.Join(rest, "\n\n---\n\n"))
}
