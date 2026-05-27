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

package transformers

import "github.com/go-openapi/spec"

// Copy is a Transformer that produces a deep clone of a schema via a
// JSON marshal/unmarshal round-trip. Used before mutations that must not
// affect the original schema object (e.g. before RequiredForHelm).
type Copy struct{}

// Transform returns a deep copy of s, leaving the original unmodified.
func (t *Copy) Transform(s *spec.Schema) *spec.Schema {
	tmpBytes, _ := s.MarshalJSON()
	res := new(spec.Schema)
	_ = res.UnmarshalJSON(tmpBytes)
	return res
}
