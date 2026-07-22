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

package crdenricher

import "testing"

// TestGenerationGolden captures the default output (example generation off): no
// synthesized root example, only the explicit channel example is applied.
func TestGenerationGolden(t *testing.T) {
	assertGolden(t, "generation.yaml", runFixture(t, "generation.yaml", false))
}

// TestGenerationExamplesGolden captures the output with example generation on:
// the root gains a synthesized example aggregating the spec fields.
func TestGenerationExamplesGolden(t *testing.T) {
	assertGolden(t, "generation.examples.yaml", runFixture(t, "generation.yaml", true))
}
