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

package types

// Always matches unconditionally (for default/fallback rules).
type Always struct{}

func (Always) Match(_ *MappingInput) bool { return true }
func (Always) String() string             { return "Always" }

// Predicate allows custom matching logic with access to full input.
type Predicate struct {
	Name string
	Fn   func(input *MappingInput) bool
}

func (p Predicate) Match(input *MappingInput) bool {
	return p.Fn(input)
}

func (p Predicate) String() string {
	return "Predicate(" + p.Name + ")"
}
