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

package bootstrap

import "github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"

// Collapse retry waits to zero for the whole test binary. The bootstrap
// flow runs a couple of "wait until cluster is in this state" loops with
// 3-15s backoffs that accumulate to ~80s when the negative-path test cases
// deliberately exhaust the budget.
func init() {
	retry.InTestEnvironment = true
}
