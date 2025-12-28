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

package statusmapper

// AllOf matches when ALL sub-matchers match (logical AND).
type AllOf []Matcher

func (a AllOf) Match(input *Input) bool {
	for _, m := range a {
		if !m.Match(input) {
			return false
		}
	}
	return true
}

func (a AllOf) String() string {
	if len(a) == 0 {
		return "AllOf()"
	}
	s := "AllOf("
	for i, m := range a {
		if i > 0 {
			s += " AND "
		}
		s += m.String()
	}
	return s + ")"
}

// AnyOf matches when ANY sub-matcher matches (logical OR).
type AnyOf []Matcher

func (a AnyOf) Match(input *Input) bool {
	for _, m := range a {
		if m.Match(input) {
			return true
		}
	}
	return false
}

func (a AnyOf) String() string {
	if len(a) == 0 {
		return "AnyOf()"
	}
	s := "AnyOf("
	for i, m := range a {
		if i > 0 {
			s += " OR "
		}
		s += m.String()
	}
	return s + ")"
}

// And combines matchers with logical AND (all must match).
func And(matchers ...Matcher) AllOf { return matchers }

// Or combines matchers with logical OR (any must match).
func Or(matchers ...Matcher) AnyOf { return matchers }
