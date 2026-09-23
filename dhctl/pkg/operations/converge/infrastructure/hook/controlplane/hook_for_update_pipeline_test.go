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
	"errors"
	"fmt"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/lib-dhctl/pkg/retry"
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
			// A "no" stubs out the readiness gate so the branch below is what the test
			// reaches; converge itself never answers no — see confirmOrProceed.
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

	err := hook.AfterAction(t.Context(), recreatedMasterRunner{}, nil)

	require.ErrorContains(t, err, "get ssh client")
}

// TestAfterActionSkipsWaitsWhenApplyFailed: the waits below the bookkeeping are for the node the
// apply was supposed to produce, and an apply that failed produced none. The converge that
// prompted this failed on plan ordering in seconds and then sat waiting for a master that no
// longer existed, reporting a second failure that only restated the first.
//
// commanderMode skips the SSH bookkeeping, leaving the fake kube client to carry the rest, so what
// the two cases differ in is only whether the waiting happens.
func TestAfterActionSkipsWaitsWhenApplyFailed(t *testing.T) {
	retry.InTestEnvironment = true
	t.Cleanup(func() { retry.InTestEnvironment = false })

	newHook := func() *HookForUpdatePipeline {
		kubeCl := client.NewFakeKubernetesClient()
		return NewHookForUpdatePipeline(
			kubernetes.NewSimpleKubeClientGetter(kubeCl),
			sshProviderWithoutClient{},
			map[string]string{"cluster-master-0": "10.12.1.10"},
			true, // commanderMode: no session to move
			true,
			false,
		).WithNodeToConverge("cluster-master-0")
	}

	applyErr := errors.New("infrastructure utility exited with code 1")
	require.NoError(t, newHook().AfterAction(t.Context(), recreatedMasterRunner{}, applyErr),
		"a failed apply must not be followed by waiting, nor by a second error restating the first")

	// Without an apply error the same call does wait - and against a cluster where the node never
	// appears, that wait is what fails. This is what the case above is skipping.
	require.Error(t, newHook().AfterAction(t.Context(), recreatedMasterRunner{}, nil),
		"a successful apply must still be followed by the readiness checks")
}
