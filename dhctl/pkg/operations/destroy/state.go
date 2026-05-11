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

import (
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/directoryconfig"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	dhctlstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

// destroyState is the per-operation bag passed through the destroy phase
// pipeline. NewClusterDestroyer builds the "before phases run" half (the
// sub-destroyers, state loader, pipeline); each phase reads what it needs
// and writes its outputs (MetaConfig, ChosenInfraDestroyer) for the next.
//
// The struct is mutated in place. Phases declared in phase_*.go must each
// document which fields they read and write.
type destroyState struct {
	// Setup-time dependencies, populated by NewClusterDestroyer and read by
	// the phases. They are immutable from the phases' point of view.
	stateCache       dhctlstate.Cache
	configPreparator metaConfigPopulator
	d8Destroyer      *deckhouse.Destroyer
	infraProvider    *infraDestroyerProvider
	pipeline         phases.DefaultPipeline
	directoryConfig  *directoryconfig.DirectoryConfig

	// Per-call inputs.
	autoApprove bool

	// Phase outputs, filled in as the pipeline runs.
	metaConfig      *config.MetaConfig
	chosenDestroyer infraDestroyer
}
