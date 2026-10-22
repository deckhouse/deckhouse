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

package dhctl

import (
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

// cachePathConfig builds the configuration that a handler parses only to name the state cache. The
// cluster prefix may live in the "global" ModuleConfig among the desired resources instead of
// ClusterConfiguration, so that ModuleConfig goes in; the other resources do not, because this parse
// has no provider bundle to validate them.
func cachePathConfig(resources string, configs ...string) string {
	return input.CombineYAMLs(append(configs, globalModuleConfig(resources))...)
}

func globalModuleConfig(resources string) string {
	for _, doc := range input.YAMLSplitRegexp.Split(strings.TrimSpace(resources), -1) {
		var head struct {
			Kind     string `json:"kind"`
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		}

		if yaml.Unmarshal([]byte(doc), &head) == nil && head.Kind == "ModuleConfig" && head.Metadata.Name == "global" {
			return doc
		}
	}

	return ""
}
