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
	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	"github.com/deckhouse/lib-connection/pkg/ssh/testssh"

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
			operatorSessionForNode,
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
		operatorSessionForNode,
		false,
		true,
		false,
	).WithNodeToConverge("cluster-master-0")

	err := hook.AfterAction(t.Context(), recreatedMasterRunner{})

	require.ErrorContains(t, err, "get ssh client")
}

// operatorSessionForNode is the mapping of a cluster this converge has not rebuilt: every
// node answers to the user the session already runs as.
func operatorSessionForNode(base *session.Session, host session.Host) (*session.Session, error) {
	return session.NewSession(session.Input{
		User:           base.User,
		Port:           base.Port,
		BecomePass:     base.BecomePass,
		AvailableHosts: []session.Host{host},
	}), nil
}

// A master this converge rebuilt answers to the converge user while the clients still run
// as the operator. Putting it in their session mixes two users into one host list, and
// every later connection that picks it fails to log in.
func TestAfterActionKeepsTheSessionToOneUser(t *testing.T) {
	const (
		oldIP = "10.12.1.1"
		newIP = "10.12.1.10"
	)

	newHook := func(userForRecreated string) (*HookForUpdatePipeline, *session.Session) {
		live := session.NewSession(session.Input{
			User: "ubuntu",
			AvailableHosts: []session.Host{
				{Host: oldIP, Name: "cluster-master-0"},
				{Host: "10.12.1.2", Name: "cluster-master-1"},
			},
		})

		hook := NewHookForUpdatePipeline(
			unreachableKubeGetter{},
			testssh.NewSSHProvider(live, true),
			map[string]string{"cluster-master-1": "10.12.1.2"},
			func(_ *session.Session, host session.Host) (*session.Session, error) {
				return session.NewSession(session.Input{
					User:           userForRecreated,
					AvailableHosts: []session.Host{host},
				}), nil
			},
			false,
			true,
			false,
		).WithNodeToConverge("cluster-master-0")

		hook.oldMasterIPForSSH = oldIP

		return hook, live
	}

	hostNames := func(sess *session.Session) []string {
		names := make([]string, 0, len(sess.AvailableHosts()))
		for _, host := range sess.AvailableHosts() {
			names = append(names, host.Host)
		}

		return names
	}

	t.Run("a node of another generation stays out", func(t *testing.T) {
		hook, live := newHook("d8-converge")

		// The kube client is unreachable, so AfterAction fails right after the move.
		require.Error(t, hook.AfterAction(t.Context(), recreatedMasterRunner{}))
		require.Equal(t, []string{"10.12.1.2"}, hostNames(live))
	})

	t.Run("a node of the session's own generation joins", func(t *testing.T) {
		hook, live := newHook("ubuntu")

		require.Error(t, hook.AfterAction(t.Context(), recreatedMasterRunner{}))
		require.ElementsMatch(t, []string{"10.12.1.2", newIP}, hostNames(live))
	})
}
