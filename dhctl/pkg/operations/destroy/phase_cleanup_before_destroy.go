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

// cleanupBeforeDestroyPhase tears down auxiliary connections that the
// remaining phases no longer need — the kube proxy tunnel, the SSH client.
// All required state has already been pulled into the cache by this point.
//
// Reads state.ChosenDestroyer.
type cleanupBeforeDestroyPhase struct{}

func (cleanupBeforeDestroyPhase) Name() string { return "cleanup-before-destroy" }

func (cleanupBeforeDestroyPhase) Run(ctx context.Context, s *destroyState) error {
	return s.ChosenDestroyer.CleanupBeforeDestroy(ctx)
}
