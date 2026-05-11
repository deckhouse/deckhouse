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

// checkCommanderUUIDPhase enforces the commander-mode UUID consistency
// check before any destructive work begins. In non-commander mode the
// underlying d8Destroyer call short-circuits to a no-op.
//
// Reads: state.d8Destroyer.
// Writes: nothing (sub-destroyer may emit phases.CommanderUUIDWasChecked
//
//	through its own PhasedActionProvider).
type checkCommanderUUIDPhase struct{}

func (checkCommanderUUIDPhase) Name() string { return "check-commander-uuid" }

func (checkCommanderUUIDPhase) Run(ctx context.Context, s *destroyState) error {
	return s.d8Destroyer.CheckCommanderUUID(ctx)
}
