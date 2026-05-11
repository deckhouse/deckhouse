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

// finalizeResourcesPhase persists "deckhouse resources deleted" into the
// state cache so a subsequent attempt resumes from the right point.
//
// Reads: state.d8Destroyer.
// Writes: nothing (sub-destroyer emits phases.SetDeckhouseResourcesDeletedPhase).
type finalizeResourcesPhase struct{}

func (finalizeResourcesPhase) Name() string { return "finalize-resources" }

func (finalizeResourcesPhase) Run(ctx context.Context, s *destroyState) error {
	return s.d8Destroyer.Finalize(ctx)
}
