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

// cleanupStateCachePhase wipes the local on-disk state cache after a
// successful destroy. Survives only the tombstone key the pipeline writes
// on completion.
//
// Reads: state.stateCache.
// Writes: clears every cache key bar the post-cleanup tombstone.
type cleanupStateCachePhase struct{}

func (cleanupStateCachePhase) Name() string { return "cleanup-state-cache" }

func (cleanupStateCachePhase) Run(ctx context.Context, s *destroyState) error {
	s.stateCache.Clean(ctx)
	return nil
}
