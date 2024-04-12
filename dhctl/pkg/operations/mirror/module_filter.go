// Copyright 2024 Flant JSC
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

package mirror

import (
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

// ModuleFilter maps module names to required tags
type ModuleFilter map[string][]string

func ParseModuleFilterString(str string) ModuleFilter {
	filter := make(ModuleFilter)
	if str == "" {
		return nil
	}

	modules := strings.Split(str, ";")
	for _, filterEntry := range modules {
		moduleName, moduleTag, validSplit := strings.Cut(strings.TrimSpace(filterEntry), ":")
		if !validSplit {
			log.WarnF("Malformed filter %q is ignored\n", filterEntry)
			continue
		}

		if filter[moduleName] == nil {
			filter[moduleName] = make([]string, 0)
		}

		filter[moduleName] = append(filter[moduleName], moduleTag)
	}

	return filter
}

func (f ModuleFilter) Match(mod Module) bool {
	if len(f) == 0 {
		return true
	}

	for _, tag := range f[mod.Name] {
		for _, release := range mod.Releases {
			if release == tag {
				return true
			}
		}
	}

	return false
}

func (f ModuleFilter) FilterModuleReleases(mod *Module) {
	mod.Releases = f[mod.Name]
}
