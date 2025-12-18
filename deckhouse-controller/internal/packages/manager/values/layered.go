// Copyright 2025 Flant JSC
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

package values

import (
	addonvalues "github.com/flant/addon-operator/pkg/utils"

	"github.com/deckhouse/module-sdk/pkg/utils"
)

// valuesTransform is a function type that transforms values based on current values.
// Used for dynamic transformations that depend on existing values.
type valuesTransform func(values addonvalues.Values) addonvalues.Values

// valuesTransformer is an interface for objects that can transform values.
// Allows for stateful transformers (e.g., with schema information).
type valuesTransformer interface {
	Transform(values addonvalues.Values) addonvalues.Values
}

// mergeLayers merges multiple value layers into a single values object.
// Layers are applied in order, with later layers overwriting earlier ones.
//
// Supported layer types:
//   - utils.Values: Merged directly
//   - map[string]interface{}: Merged directly
//   - string: Parsed as YAML/JSON and merged
//   - valuesTransform: Function called with current values
//   - valuesTransformer: Interface Transform() method called
//   - nil: Skipped
func mergeLayers(initial addonvalues.Values, layers ...interface{}) addonvalues.Values {
	res := addonvalues.MergeValues(initial)

	for _, layer := range layers {
		switch l := layer.(type) {
		case addonvalues.Values:
			res = addonvalues.MergeValues(res, l)
		case map[string]interface{}:
			res = addonvalues.MergeValues(res, l)
		case string:
			tmp, _ := utils.NewValuesFromBytes([]byte(l))
			res = addonvalues.MergeValues(res, tmp)
		case valuesTransform:
			// Call transform function with current values
			res = addonvalues.MergeValues(res, l(res))
		case valuesTransformer:
			// Call Transform method on transformer interface
			res = addonvalues.MergeValues(res, l.Transform(res))
		case nil:
			// Skip nil layers
			continue
		}
	}

	return res
}
