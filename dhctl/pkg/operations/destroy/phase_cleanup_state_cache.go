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

	dhctlstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

// cleanupStateCachePhase wipes the local on-disk state cache after a
// successful destroy. Only the tombstone the pipeline writes on completion
// survives.
type cleanupStateCachePhase struct {
	stateCache dhctlstate.Cache
}

func (p cleanupStateCachePhase) Name() string { return "cleanup-state-cache" }

func (p cleanupStateCachePhase) Run(ctx context.Context, _ *destroyState) error {
	p.stateCache.Clean(ctx)
	return nil
}
