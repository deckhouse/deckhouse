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

package client

import (
	"context"
	"testing"

	libcon "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg"
	"github.com/stretchr/testify/require"
)

type fakeSSHClient struct{ libcon.SSHClient }

// TestTheNodeConnectionIsOpenedOnlyWhenAskedFor: a client given a way to connect opens nothing until a
// caller asks for the connection, and one that already carries a connection uses it.
func TestTheNodeConnectionIsOpenedOnlyWhenAskedFor(t *testing.T) {
	opened := &fakeSSHClient{}
	calls := 0
	connect := func(context.Context) (libcon.SSHClient, error) {
		calls++
		return opened, nil
	}

	kubeCl := NewFakeKubernetesClient().WithSSHConnect(connect)
	require.True(t, kubeCl.CanReachNodes())
	require.Zero(t, calls)

	got, err := kubeCl.NodeSSHClient(t.Context())
	require.NoError(t, err)
	require.Same(t, opened, got)
	require.Equal(t, 1, calls)

	carried := &fakeSSHClient{}
	got, err = NewFakeKubernetesClient().WithSSHClient(carried).WithSSHConnect(connect).NodeSSHClient(t.Context())
	require.NoError(t, err)
	require.Same(t, carried, got)
	require.Equal(t, 1, calls)

	none := NewFakeKubernetesClient().WithSSHConnect(nil)
	require.False(t, none.CanReachNodes())
	got, err = none.NodeSSHClient(t.Context())
	require.NoError(t, err)
	require.Nil(t, got)
}
