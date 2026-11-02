/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ledger

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// A leader resumed after a pause longer than the fence still has its leadership context until the
// renew deadline; it must not write the route of the ledger, which another replica may hold by then.
func TestLeaderActsOnlyWithinTheFence(t *testing.T) {
	clock := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	lock := &FencedLock{Interface: &slowLock{clock: &clock}, Now: func() time.Time { return clock }}
	r := &runner{timing: DefaultTiming, lock: lock}
	r.synced.Store(true)
	require.NoError(t, lock.Update(context.Background(), resourcelock.LeaderElectionRecord{HolderIdentity: "me"}))
	assert.True(t, r.mayAct())

	clock = clock.Add(25 * time.Second)
	assert.False(t, r.mayAct(), "paused past the fence")

	require.NoError(t, lock.Update(context.Background(), resourcelock.LeaderElectionRecord{HolderIdentity: "me"}))
	assert.True(t, r.mayAct())
	r.stopping.Store(true)
	assert.False(t, r.mayAct(), "a stopping leader withdrew its address")
}
