/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v2

import (
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// Every hook of this package writes `internal.v2` as a whole, so a value the schema declares but
// Values does not carry is erased by whichever hook runs next. That is how the aggregated API's
// certificate vanished on a cluster and left registry-controller in ContainerCreating: the
// certificate hook wrote it, the next hook dropped it, and the Secret was never rendered.
func TestValuesCarryEverySchemaProperty(t *testing.T) {
	raw, err := os.ReadFile("../../openapi/values.yaml")
	require.NoError(t, err)

	var schema struct {
		Properties struct {
			Internal struct {
				Properties struct {
					V2 struct {
						Properties map[string]any `json:"properties"`
					} `json:"v2"`
				} `json:"properties"`
			} `json:"internal"`
		} `json:"properties"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &schema))

	declared := schema.Properties.Internal.Properties.V2.Properties
	require.NotEmpty(t, declared, "internal.v2 has no properties in the schema")

	carried := map[string]bool{}
	typ := reflect.TypeOf(Values{})
	for i := range typ.NumField() {
		name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
		carried[name] = true
	}

	var missing []string
	for name := range declared {
		if !carried[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	require.Empty(t, missing, "internal.v2 values that Values does not carry, and the hooks erase")
}
