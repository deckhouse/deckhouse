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

// TestReindentGolden captures the output with the reindent flag on: the whole
// document uses the goyaml.v3 indented block-sequence layout. The bare fixture
// carries no ordered examples, so this isolates the layout change (indented
// sequences) from the order-preserving behaviour.
func TestReindentGolden(t *testing.T) {
	assertGolden(t, "bare.reindent.yaml", runFixtureOpts(t, "bare.yaml", Options{Reindent: true}))
}

// TestReindentOrderedGolden captures reindent combined with an ordered example:
// block sequences are indented and the example still keeps its authored key
// order, so the two behaviours compose.
func TestReindentOrderedGolden(t *testing.T) {
	assertGolden(t, "ordered.reindent.yaml", runFixtureOpts(t, "ordered.yaml", Options{Reindent: true}))
}
