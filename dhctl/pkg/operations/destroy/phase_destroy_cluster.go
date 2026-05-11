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

// destroyClusterPhase performs the actual destructive teardown: cloud
// destroyer runs `tofu destroy`, static destroyer SSHes into the masters
// and runs the cleanup script. Sub-destroyer emits phases.AllNodesPhase
// (static) or its own cloud-side markers.
//
// Reads state.ChosenDestroyer, state.AutoApprove.
type destroyClusterPhase struct{}

func (destroyClusterPhase) Name() string { return "destroy-cluster" }

func (destroyClusterPhase) Run(ctx context.Context, s *destroyState) error {
	return s.ChosenDestroyer.DestroyCluster(ctx, s.AutoApprove)
}
