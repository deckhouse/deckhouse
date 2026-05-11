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
	"context"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
)

// chooseDestroyerPhase picks the cloud or static destroyer implementation
// based on metaConfig.ClusterType and tags the surrounding pipeline so
// commander/progress consumers can branch on it.
//
// Reads: state.metaConfig, state.infraProvider, state.pipeline.
// Writes: state.chosenDestroyer.
type chooseDestroyerPhase struct{}

func (chooseDestroyerPhase) Name() string { return "choose-destroyer" }

func (chooseDestroyerPhase) Run(ctx context.Context, s *destroyState) error {
	d, err := config.DoByClusterType(ctx, s.metaConfig, s.infraProvider)
	if err != nil {
		return err
	}
	s.chosenDestroyer = d
	s.pipeline.SetClusterConfig(phases.ClusterConfig{ClusterType: s.metaConfig.ClusterType})
	return nil
}
