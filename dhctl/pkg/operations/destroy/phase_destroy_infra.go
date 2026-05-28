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

// destroyInfraPhase is the final destructive phase: tears down the
// underlying infrastructure (tofu destroy for cloud, master-node cleanup
// scripts for static). By this point the k8s API access from
// deleteResourcesPhase has already been closed; static reopens a fresh
// SSH session via SSHClientProvider for the cleanup scripts.
type destroyInfraPhase struct{}

func (destroyInfraPhase) run(ctx context.Context, prep prepared, autoApprove bool) error {
	return prep.destroyer.DestroyCluster(ctx, autoApprove)
}
