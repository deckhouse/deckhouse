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

// prepareDestroyerPhase runs provider-specific preparation: cloud destroyer
// locks the converge lease; static destroyer creates the d8-dhctl-converger
// node user and waits for it to land on the masters.
//
// Reads: state.chosenDestroyer.
// Writes: nothing observable at this layer (sub-destroyer emits its own
//
//	tracked phases through PhasedActionProvider).
type prepareDestroyerPhase struct{}

func (prepareDestroyerPhase) Name() string { return "prepare-destroyer" }

func (prepareDestroyerPhase) Run(ctx context.Context, s *destroyState) error {
	return s.chosenDestroyer.Prepare(ctx)
}
