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
	"github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
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

// A master this converge rebuilt answers to the converge user — this hook runs on nothing
// else. Putting it in a session that runs as the operator mixes two users into one host
// list, and every later connection that picks it fails to log in.
func TestAfterActionKeepsTheSessionToOneUser(t *testing.T) {
	const (
		oldIP  = "10.12.1.1"
		peerIP = "10.12.1.2"
		newIP  = "10.12.1.10"
	)

	newHook := func(liveUser string) (*HookForUpdatePipeline, *session.Session) {
		live := session.NewSession(session.Input{
			User: liveUser,
			AvailableHosts: []session.Host{
				{Host: oldIP, Name: "cluster-master-0"},
				{Host: peerIP, Name: "cluster-master-1"},
			},
		})

		hook := NewHookForUpdatePipeline(
			unreachableKubeGetter{},
			testssh.NewSSHProvider(live, true),
			map[string]string{"cluster-master-1": peerIP},
			operatorSessionForNode,
			false,
			true,
			false,
		).WithNodeToConverge("cluster-master-0")

		hook.oldMasterIPForSSH = oldIP

		return hook, live
	}

	addresses := func(sess *session.Session) []string {
		hosts := make([]string, 0, len(sess.AvailableHosts()))
		for _, host := range sess.AvailableHosts() {
			hosts = append(hosts, host.Host)
		}

		return hosts
	}

	t.Run("the clients run as the operator, so the rebuilt node stays out", func(t *testing.T) {
		hook, live := newHook("ubuntu")

		// The kube client is unreachable, so AfterAction fails right after the move.
		require.Error(t, hook.AfterAction(t.Context(), recreatedMasterRunner{}))
		require.Equal(t, []string{peerIP}, addresses(live))
	})

	t.Run("the clients already run as the converge user, so it joins", func(t *testing.T) {
		hook, live := newHook(global.ConvergeUserName)

		require.Error(t, hook.AfterAction(t.Context(), recreatedMasterRunner{}))
		require.ElementsMatch(t, []string{peerIP, newIP}, addresses(live))
	})
}

// The checker caches one client per node, and the legacy backend reports a cached client
// alive forever. Rebuilding the VM behind a name leaves that client pointed at a machine
// that no longer exists, under the user the old one answered to.
func TestAfterActionDropsTheCachedCheckClient(t *testing.T) {
	const (
		oldIP = "10.12.1.1"
		newIP = "10.12.1.10"
	)

	live := session.NewSession(session.Input{
		User:           "ubuntu",
		AvailableHosts: []session.Host{{Host: oldIP, Name: "cluster-master-0"}},
	})

	provider := testssh.NewSSHProvider(live, true)

	ran := map[string]int{}
	for _, address := range []string{oldIP, newIP} {
		ran[address] = 0

		provider.AddCommandProvider(address, func(_ testssh.Bastion, _ string, _ ...string) *testssh.Command {
			return testssh.NewCommand(nil).WithRun(func() { ran[address]++ })
		})
	}

	sessionForNode := func(base *session.Session, host session.Host) (*session.Session, error) {
		return session.NewSession(session.Input{User: base.User, AvailableHosts: []session.Host{host}}), nil
	}

	// A peer check before the rebuild: it caches a client keyed by the node name.
	ready, err := NewSSHChecker(provider, map[string]string{"cluster-master-0": oldIP}, sessionForNode).
		IsReady(t.Context(), "cluster-master-0")
	require.NoError(t, err)
	require.True(t, ready)

	hook := NewHookForUpdatePipeline(
		unreachableKubeGetter{},
		provider,
		map[string]string{"cluster-master-1": "10.12.1.2"},
		sessionForNode,
		false,
		true,
		false,
	).WithNodeToConverge("cluster-master-0")

	require.Error(t, hook.AfterAction(t.Context(), recreatedMasterRunner{}))

	// The same node, now at the address it was rebuilt on.
	ready, err = NewSSHChecker(provider, map[string]string{"cluster-master-0": newIP}, sessionForNode).
		IsReady(t.Context(), "cluster-master-0")
	require.NoError(t, err)
	require.True(t, ready)

	require.Equal(t, map[string]int{oldIP: 1, newIP: 1}, ran,
		"the check after the rebuild must reach the new machine, not the cached client")
}

// The rebuilt master is the clients' last host, and a provider that drops the account from
// its cloud-config leaves them there under a user it does not know. The switch itself
// reports nothing on the legacy backend, so only a command on it tells the two apart.
func TestAfterActionProvesTheRecreatedNodeAnswers(t *testing.T) {
	const (
		oldIP        = "10.12.1.1"
		newIP        = "10.12.1.10"
		operatorSudo = "operator-sudo"
	)

	newProvider := func(t *testing.T, accepted string, ran *[]string) *testssh.SSHProvider {
		t.Helper()

		live := session.NewSession(session.Input{
			User:           "ubuntu",
			BecomePass:     operatorSudo,
			AvailableHosts: []session.Host{{Host: oldIP, Name: "cluster-master-0"}},
		})

		provider := testssh.NewSSHProvider(live, true)
		provider.AddCommandProvider(newIP, func(_ testssh.Bastion, _ string, _ ...string) *testssh.Command {
			switches := provider.Switches()
			user := switches[len(switches)-1].Session.User
			*ran = append(*ran, user)

			if user != accepted {
				return testssh.NewCommand(nil).WithErr(fmt.Errorf("Permission denied (publickey)"))
			}

			return testssh.NewCommand(nil)
		})

		return provider
	}

	move := func(t *testing.T, provider *testssh.SSHProvider) error {
		t.Helper()

		hook := NewHookForUpdatePipeline(
			unreachableKubeGetter{},
			provider,
			map[string]string{"cluster-master-1": "10.12.1.2"},
			operatorSessionForNode,
			false,
			true,
			false,
		).WithNodeToConverge("cluster-master-0")

		hook.oldMasterIPForSSH = oldIP

		cl, err := provider.Client(t.Context())
		require.NoError(t, err)

		return hook.moveSessionToRecreatedNode(t.Context(), cl, session.Host{Host: newIP, Name: "cluster-master-0"})
	}

	t.Run("the account is there", func(t *testing.T) {
		var ran []string

		require.NoError(t, move(t, newProvider(t, global.ConvergeUserName, &ran)))
		require.Equal(t, []string{global.ConvergeUserName}, ran)
	})

	t.Run("the provider dropped it, so the operator takes over", func(t *testing.T) {
		var ran []string

		provider := newProvider(t, "ubuntu", &ran)

		require.NoError(t, move(t, provider))
		require.Equal(t, []string{global.ConvergeUserName, "ubuntu"}, ran,
			"a rejected converge user must be retried as the user dhctl started with")

		// The sudo password the operator gave belongs to the operator's account. The
		// converge user's sudo is NOPASSWD, and a credential nothing reads is still one
		// that was sent to another account.
		switches := provider.Switches()
		require.Len(t, switches, 2)
		require.Equal(t, global.ConvergeUserName, switches[0].Session.User)
		require.Empty(t, switches[0].Session.BecomePass,
			"the converge user must be reached with no sudo password")
		require.Equal(t, "ubuntu", switches[1].Session.User)
		require.Equal(t, operatorSudo, switches[1].Session.BecomePass,
			"the operator fallback must keep the sudo password dhctl was started with")
	})

	// A machine the provider reports as running has not finished booting: sshd and
	// cloud-init land later, and until then both accounts are refused alike. Reading that
	// as a node that lost the converge user ends the converge on a node that was coming up.
	t.Run("the node is still booting", func(t *testing.T) {
		var ran []string

		live := session.NewSession(session.Input{
			User:           "ubuntu",
			BecomePass:     operatorSudo,
			AvailableHosts: []session.Host{{Host: oldIP, Name: "cluster-master-0"}},
		})

		provider := testssh.NewSSHProvider(live, true)

		// One full pass over both accounts before sshd answers anybody.
		refusals := 2

		provider.AddCommandProvider(newIP, func(_ testssh.Bastion, _ string, _ ...string) *testssh.Command {
			switches := provider.Switches()
			user := switches[len(switches)-1].Session.User
			ran = append(ran, user)

			if refusals > 0 {
				refusals--

				return testssh.NewCommand(nil).WithErr(fmt.Errorf("exit status 255"))
			}

			if user != global.ConvergeUserName {
				return testssh.NewCommand(nil).WithErr(fmt.Errorf("Permission denied (publickey)"))
			}

			return testssh.NewCommand(nil)
		})

		require.NoError(t, move(t, provider))
		require.Equal(t, []string{global.ConvergeUserName, "ubuntu", global.ConvergeUserName}, ran,
			"a node that answers nobody yet must be waited for, not written off")
	})

	t.Run("neither answers", func(t *testing.T) {
		// The wait is what this subtest exhausts, so it runs on a single attempt.
		retry.InTestEnvironment = true
		t.Cleanup(func() { retry.InTestEnvironment = false })

		var ran []string

		err := move(t, newProvider(t, "nobody", &ran))

		require.ErrorContains(t, err, global.ConvergeUserName)
		require.ErrorContains(t, err, "ubuntu")
	})
}
