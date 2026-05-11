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

package destroy

import "context"

// afterResourcesDeletePhase runs the provider hook executed once the
// deckhouse-managed resources are gone but before the infrastructure
// itself is torn down. For static clusters it waits for the destroyer
// node user to disappear from every master.
//
// Reads: state.chosenDestroyer.
// Writes: nothing.
type afterResourcesDeletePhase struct{}

func (afterResourcesDeletePhase) Name() string { return "after-resources-delete" }

func (afterResourcesDeletePhase) Run(ctx context.Context, s *destroyState) error {
	return s.chosenDestroyer.AfterResourcesDelete(ctx)
}
