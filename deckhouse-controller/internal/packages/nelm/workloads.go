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

package nelm

import (
	"strings"

	"k8s.io/apimachinery/pkg/util/yaml"
)

// workloadAPIVersion is the group version of every kind the health monitor watches.
const workloadAPIVersion = "apps/v1"

// workloadKinds must match the informers registered by health/monitor.
var workloadKinds = map[string]struct{}{
	"Deployment":  {},
	"StatefulSet": {},
}

// typeMeta is the minimal projection of a manifest needed to classify it.
type typeMeta struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
}

// hasWorkloads reports whether the rendered multi-document YAML manifest holds a
// workload the health monitor observes. Decoding stops at the first undecodable document.
func hasWorkloads(renderedManifests string) bool {
	dec := yaml.NewYAMLOrJSONDecoder(strings.NewReader(renderedManifests), 4096)

	for {
		meta := new(typeMeta)
		// The JSON decoder returns a syntax error sticky, so skipping it would spin forever.
		// Reporting none is safe: the monitor's first event overrides NoWorkloads.
		if err := dec.Decode(meta); err != nil {
			return false
		}

		if _, ok := workloadKinds[meta.Kind]; ok && meta.APIVersion == workloadAPIVersion {
			return true
		}
	}
}
