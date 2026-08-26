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

package controlplane

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	libcon "github.com/deckhouse/lib-connection/pkg"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/plan"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

// destroyedMasterRunner reports what a converge interrupted after the VM was
// destroyed finds on restart: the plan still wants the machine back, but no output
// holds an address any more.
type destroyedMasterRunner struct {
	infrastructure.RunnerInterface
}

func (destroyedMasterRunner) GetChangesInPlan() int { return plan.HasDestructiveChanges }

func (destroyedMasterRunner) HasVMDestruction() bool { return true }

func (destroyedMasterRunner) GetInfrastructureOutput(_ context.Context, _ string) ([]byte, error) {
	return []byte(`""`), nil
}

func (destroyedMasterRunner) GetState() ([]byte, error) { return []byte("{}"), nil }

type unreachableKubeGetter struct{}

func (unreachableKubeGetter) KubeClientCtx(_ context.Context) (*client.KubernetesClient, error) {
	return nil, fmt.Errorf("cluster is unreachable")
}

// An immutable node is retired over the Kubernetes API alone, so a missing SSH
// address is no reason to skip: skipping leaves it a voting etcd member while the VM
// behind it is recreated. Reaching the kube client is what proves the hook went on.
func TestBeforeActionRetiresImmutableMasterWithoutSSHIP(t *testing.T) {
	newHook := func(immutableNode bool) *HookForUpdatePipeline {
		return NewHookForUpdatePipeline(
			unreachableKubeGetter{},
			nil,
			map[string]string{"cluster-master-1": ""},
			false,
			true,
			immutableNode,
		).
			WithNodeToConverge("cluster-master-0").
			WithConfirm(func(_ string) bool { return false })
	}

	t.Run("immutable", func(t *testing.T) {
		_, err := newHook(true).BeforeAction(t.Context(), destroyedMasterRunner{})

		require.ErrorContains(t, err, "cluster is unreachable")
	})

	t.Run("bashible", func(t *testing.T) {
		_, err := newHook(false).BeforeAction(t.Context(), destroyedMasterRunner{})

		require.NoError(t, err)
	})
}

// recreatedMasterRunner reports the one plan shape that makes AfterAction do its
// work: the master VM was destroyed and created again.
type recreatedMasterRunner struct {
	infrastructure.RunnerInterface
}

func (recreatedMasterRunner) GetChangesInPlan() int { return plan.HasDestructiveChanges }

func (recreatedMasterRunner) HasVMDestruction() bool { return true }

func (recreatedMasterRunner) GetInfrastructureOutput(_ context.Context, _ string) ([]byte, error) {
	return []byte(`"10.12.1.10"`), nil
}

func (recreatedMasterRunner) GetState() ([]byte, error) { return []byte("{}"), nil }

type sshProviderWithoutClient struct {
	libcon.SSHProvider
}

func (sshProviderWithoutClient) Client(_ context.Context) (libcon.SSHClient, error) {
	return nil, fmt.Errorf("no ssh hosts")
}

func TestAfterActionReportsUnavailableSSHClient(t *testing.T) {
	hook := NewHookForUpdatePipeline(
		nil,
		sshProviderWithoutClient{},
		map[string]string{"cluster-master-0": "10.12.1.10"},
		false,
		true,
		false,
	).WithNodeToConverge("cluster-master-0")

	err := hook.AfterAction(t.Context(), recreatedMasterRunner{})

	require.ErrorContains(t, err, "get ssh client")
}
