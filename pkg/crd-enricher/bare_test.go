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

// TestBareGolden captures examples without a description: x-doc-examples stays a
// plain list of values — scalars stay scalars and objects stay bare objects,
// never a wrapper.
func TestBareGolden(t *testing.T) {
	assertGolden(t, "bare.yaml", runFixture(t, "bare.yaml", false))
}
