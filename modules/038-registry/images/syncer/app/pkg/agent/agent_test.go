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

package agent

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_Defaults(t *testing.T) {
	c, err := LoadConfig([]byte(`
leaderAddress: registry-cache-leader.d8-system.svc:5001
localAddress: 127.0.0.1:5001
readUser: { name: ro, password: rp }
writeUser: { name: rw, password: wp }
ca: PEM
`))
	require.NoError(t, err)
	assert.Equal(t, 60, c.SyncIntervalSeconds)
	assert.Equal(t, 3600, c.GCIntervalSeconds)
	assert.Equal(t, "/registry", c.RegistryBinary)
	assert.Equal(t, "/config/config.yaml", c.DistributionConfig)
	assert.Equal(t, "rw", c.WriteUser.Name)
}

func TestRunSyncOnce_LeaderSkips(t *testing.T) {
	var leader atomic.Bool
	leader.Store(true)
	a := New(slog.New(slog.NewTextHandler(io.Discard, nil)), Config{
		LeaderAddress: "127.0.0.1:1", LocalAddress: "127.0.0.1:1",
	}, &leader)
	// Leader must skip without dialing anything → no error despite bogus addrs.
	require.NoError(t, a.RunSyncOnce(context.Background()))
}

func TestSyncConfig_PruneEnabled(t *testing.T) {
	var leader atomic.Bool
	a := New(slog.New(slog.NewTextHandler(io.Discard, nil)), Config{
		LeaderAddress: "L:5001", LocalAddress: "127.0.0.1:5001",
		ReadUser: User{Name: "ro", Password: "rp"}, WriteUser: User{Name: "rw", Password: "wp"}, CA: "PEM",
	}, &leader)
	sc := a.syncConfig()
	assert.True(t, sc.Prune)
	assert.Equal(t, "L:5001", sc.Src.Address)
	assert.Equal(t, "ro", sc.Src.User.Name)
	assert.Equal(t, "rw", sc.Dest.User.Name)
}
