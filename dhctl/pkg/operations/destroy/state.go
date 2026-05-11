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

import "github.com/deckhouse/deckhouse/dhctl/pkg/config"

// destroyState carries only the data that flows between destroy phases.
// Setup-time dependencies (sub-destroyers, the state cache, the pipeline,
// …) are constructor-injected onto each phase struct instead — that keeps
// every phase's contract visible from its own type declaration.
//
//   - AutoApprove: per-call input set by DestroyCluster.
//   - MetaConfig:  produced by populateMetaConfigPhase, consumed by
//     chooseDestroyerPhase.
//   - ChosenDestroyer: produced by chooseDestroyerPhase, consumed by
//     prepareDestroyerPhase / afterResourcesDeletePhase /
//     cleanupBeforeDestroyPhase / destroyClusterPhase.
type destroyState struct {
	AutoApprove     bool
	MetaConfig      *config.MetaConfig
	ChosenDestroyer infraDestroyer
}
