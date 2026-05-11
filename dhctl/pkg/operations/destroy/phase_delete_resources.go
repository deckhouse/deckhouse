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

// deleteResourcesPhase asks the deckhouse destroyer to delete user
// resources (Services, Ingresses, PVCs, etc.) from the cluster before the
// infrastructure-level teardown begins. The skip-resources flag short-circuits
// this inside d8Destroyer.
//
// Reads: state.d8Destroyer.
// Writes: nothing (sub-destroyer emits phases.DeleteResourcesPhase).
type deleteResourcesPhase struct{}

func (deleteResourcesPhase) Name() string { return "delete-resources" }

func (deleteResourcesPhase) Run(ctx context.Context, s *destroyState) error {
	return s.d8Destroyer.CheckAndDeleteResources(ctx)
}
