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

package controller

import (
	gocontext "context"
	"encoding/base64"
	"testing"
	"time"

	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/settings"
	sshconfig "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/ssh/config"
	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/ssh/session"
	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/retry"
	klient "github.com/flant/kube-client/client"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/kubeerrors"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/context"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infrastructure/hook/controlplane"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

// Without SSH hosts the hook cannot be built. Reporting that instead of a nil hook
// is what keeps the runner from silently falling back to DummyHook and recreating a
// master VM that still holds its etcd membership and its Node object.
func TestNewHookForUpdatePipelineFailsWithoutSSHHosts(t *testing.T) {
	noHosts := &sshconfig.ConnectionConfig{Config: &sshconfig.Config{}}
	convergeCtx := context.NewContext(t.Context(), context.Params{
		SSHProviderInitializer: providerinitializer.NewSSHProviderInitializer(
			settings.NewBaseProviders(settings.ProviderParams{}),
			noHosts,
		),
	})

	controller := NewMasterNodeGroupController(
		NewNodeGroupController("master", state.NodeGroupInfrastructureState{}, nil, nil),
		false,
	)

	hook, err := controller.newHookForUpdatePipeline(convergeCtx, "cluster-master-0")

	require.Error(t, err)
	require.Nil(t, hook)
}

// A run started with kube flags and unreadable SSH keys carries no provider initializer
// at all. The host map then stays empty, and an empty map is how a single-master cluster
// is recognised: a destructive plan would scale 1→3→1 and queue the two healthy masters
// for destruction.
func TestNewHookForUpdatePipelineFailsWithoutSSHConfiguration(t *testing.T) {
	convergeCtx := context.NewContext(t.Context(), context.Params{})

	controller := NewMasterNodeGroupController(
		NewNodeGroupController("master", state.NodeGroupInfrastructureState{
			State: map[string][]byte{
				"cluster-master-0": nil,
				"cluster-master-1": nil,
				"cluster-master-2": nil,
			},
		}, nil, nil),
		false,
	)

	hook, err := controller.newHookForUpdatePipeline(convergeCtx, "cluster-master-0")

	require.Error(t, err)
	require.Nil(t, hook)
}

// The readiness gate asks one question per surviving master, and an answer of "no"
// skips the check instead of failing it. With no terminal that answer defaulted to no,
// so a master was retired with the others' readiness never looked at.
func TestUpdatePipelineChecksSurvivingMastersWithoutTerminal(t *testing.T) {
	convergeCtx := context.NewContext(t.Context(), context.Params{
		KubeProvider: unreachableKubeProvider{},
	})

	nodeGroup := NewNodeGroupController("master", state.NodeGroupInfrastructureState{
		State: map[string][]byte{"cluster-master-0": nil, "cluster-master-1": nil},
	}, nil, nil)
	nodeGroup.immutable = true

	hook, err := NewMasterNodeGroupController(nodeGroup, false).
		newHookForUpdatePipeline(convergeCtx, "cluster-master-0")
	require.NoError(t, err)

	pipeline, ok := hook.(*controlplane.HookForUpdatePipeline)
	require.True(t, ok, "the update pipeline hook carries the readiness gate")

	// Cancelled so the readiness loop gives up on the unreachable cluster at once.
	checkCtx, cancel := gocontext.WithCancel(t.Context())
	cancel()

	require.Error(t, pipeline.IsAllNodesReady(checkCtx),
		"cluster-master-1 was reported ready without a single check")
}

// An immutable master is never registered as an SSH host, so the provider lookup
// fails on a master-hosts cache nobody wrote. Reporting that aborted every converge
// of an immutable master before it could render the node's payload.
func TestNewHookForUpdatePipelineNeedsNoSSHForImmutableMaster(t *testing.T) {
	noHosts := &sshconfig.ConnectionConfig{Config: &sshconfig.Config{}}
	convergeCtx := context.NewContext(t.Context(), context.Params{
		SSHProviderInitializer: providerinitializer.NewSSHProviderInitializer(
			settings.NewBaseProviders(settings.ProviderParams{}),
			noHosts,
		),
	})

	nodeGroup := NewNodeGroupController("master", state.NodeGroupInfrastructureState{
		State: map[string][]byte{"cluster-master-0": nil, "cluster-master-1": nil},
	}, nil, nil)
	nodeGroup.immutable = true

	hook, err := NewMasterNodeGroupController(nodeGroup, false).
		newHookForUpdatePipeline(convergeCtx, "cluster-master-0")

	require.NoError(t, err)
	require.NotNil(t, hook)
}

// The cached address is what makes Context.SSHless() false. Caching an immutable
// master sends the next converge looking for a NodeUser and bashible on a machine
// that runs neither.
func TestMasterHostsCacheSkipsImmutableNodes(t *testing.T) {
	newHost := []session.Host{{Host: "10.12.1.10", Name: "cluster-master-0"}}

	t.Run("immutable", func(t *testing.T) {
		stateCache := cache.NewTestCache()
		convergeCtx := context.NewContext(t.Context(), context.Params{Cache: stateCache})

		nodeGroup := NewNodeGroupController("master", state.NodeGroupInfrastructureState{}, nil, nil)
		nodeGroup.immutable = true

		NewMasterNodeGroupController(nodeGroup, false).addNewNodesToCache(convergeCtx, newHost)

		hosts, err := state.GetMasterHostsIPs(t.Context(), stateCache)
		require.NoError(t, err)
		require.Empty(t, hosts)
	})

	t.Run("bashible", func(t *testing.T) {
		stateCache := cache.NewTestCache()
		convergeCtx := context.NewContext(t.Context(), context.Params{Cache: stateCache})

		nodeGroup := NewNodeGroupController("master", state.NodeGroupInfrastructureState{}, nil, nil)

		NewMasterNodeGroupController(nodeGroup, false).addNewNodesToCache(convergeCtx, newHost)

		hosts, err := state.GetMasterHostsIPs(t.Context(), stateCache)
		require.NoError(t, err)
		require.Equal(t, newHost, hosts)
	})
}

// The map of the other masters is what tells a multi-master cluster from a single-master
// one: an empty one sends every destructive plan into the 1→3→1 scale dance. Immutable
// masters have no SSH addresses to fill it with, so it is filled by name.
func TestPopulateNodeToHostListsImmutableMastersByName(t *testing.T) {
	convergeCtx := context.NewContext(t.Context(), context.Params{})

	nodeGroup := NewNodeGroupController("master", state.NodeGroupInfrastructureState{
		State: map[string][]byte{
			"cluster-master-0": nil,
			"cluster-master-1": nil,
			"cluster-master-2": nil,
		},
	}, nil, nil)
	nodeGroup.immutable = true

	controller := NewMasterNodeGroupController(nodeGroup, false)

	require.NoError(t, controller.populateNodeToHost(convergeCtx))
	require.Equal(t,
		map[string]string{"cluster-master-0": "", "cluster-master-1": "", "cluster-master-2": ""},
		controller.nodeToHost)
}

// CheckSSHHosts asks to confirm the node-to-host mapping on every run, destructive plan
// or not. Answering "no" for a caller that has no terminal cost the whole hook, so the
// master was recreated with no guard; the answer is now yes plus a log line. Tests run
// without a TTY, which is exactly the case under test.
func TestHostsMappingConfirmedWithoutTerminal(t *testing.T) {
	convergeCtx := context.NewContext(t.Context(), context.Params{})

	require.True(t, confirmOrProceed(convergeCtx)("master-0 -> 10.12.1.10"))
}

func TestSurvivingMasterNodeNames(t *testing.T) {
	controller := NewMasterNodeGroupController(
		NewNodeGroupController("master", state.NodeGroupInfrastructureState{
			State: map[string][]byte{
				"cluster-master-2": nil,
				"cluster-master-0": nil,
				"cluster-master-1": nil,
			},
		}, nil, nil),
		false,
	)

	require.Equal(t,
		[]string{"cluster-master-0", "cluster-master-2"},
		controller.survivingMasterNodeNames("cluster-master-1"))
}

// A single master has no surviving endpoint to put in refreshed cloud-init.
// Its destructive-plan hook must trigger the existing 1→3→1 workflow first.
func TestSingleMasterHasNoVariablesRefresher(t *testing.T) {
	controller := NewMasterNodeGroupController(
		NewNodeGroupController("master", state.NodeGroupInfrastructureState{
			State: map[string][]byte{"cluster-master-0": nil},
		}, nil, nil),
		false,
	)

	require.Nil(t, controller.makeMasterNodeVariablesRefresher(nil, nil, "cluster-master-0", 0, 1, 0))
}

func newAPIServerPod(nodeName, ip string, ready corev1.ConditionStatus) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-apiserver-" + nodeName,
			Namespace: "kube-system",
			Labels:    map[string]string{"component": "kube-apiserver", "tier": "control-plane"},
		},
		Spec: corev1.PodSpec{NodeName: nodeName, HostNetwork: true},
		Status: corev1.PodStatus{
			PodIP:      ip,
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: ready}},
		},
	}
}

// newRefresherTestController returns a three-master controller and a fake cluster
// with the masters' apiserver Pods and the manual-bootstrap-for-master Secret.
// cluster-master-0 is the one being replaced: its own apiserver is not required.
func newRefresherTestController(
	t *testing.T,
	master2Ready corev1.ConditionStatus,
	secretEndpoints string,
) (*MasterNodeGroupController, *context.Context, *k8sfake.Clientset) {
	t.Helper()

	kubeCl := client.NewFakeKubernetesClient()
	for _, pod := range []*corev1.Pod{
		newAPIServerPod("cluster-master-0", "10.0.0.10", corev1.ConditionFalse),
		newAPIServerPod("cluster-master-1", "10.0.0.11", corev1.ConditionTrue),
		newAPIServerPod("cluster-master-2", "10.0.0.12", master2Ready),
	} {
		_, err := kubeCl.CoreV1().Pods(pod.Namespace).Create(t.Context(), pod, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	_, err := kubeCl.CoreV1().Secrets("d8-cloud-instance-manager").Create(t.Context(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "manual-bootstrap-for-master", Namespace: "d8-cloud-instance-manager"},
		Data: map[string][]byte{
			"apiserverEndpoints": []byte(secretEndpoints),
			"cloud-config":       []byte("#cloud-config"),
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	clientset, ok := kubeCl.KubeClient.(*klient.Client).Interface.(*k8sfake.Clientset)
	require.True(t, ok, "fake kube client is not backed by a fake clientset")

	convergeCtx := context.NewContext(t.Context(), context.Params{
		KubeProvider: staticKubeProvider{client: execCapableClient{kubeCl.KubeClient}},
	})

	nodeGroup := NewNodeGroupController("master", state.NodeGroupInfrastructureState{
		State: map[string][]byte{"cluster-master-0": nil, "cluster-master-1": nil, "cluster-master-2": nil},
	}, nil, nil)
	nodeGroup.cloudConfig = "old cloud config"

	return NewMasterNodeGroupController(nodeGroup, false), convergeCtx, clientset
}

const bothSurvivorsEndpoints = "- 10.0.0.11:6443\n- 10.0.0.12:6443\n"

func TestMasterNodeVariablesRefresher(t *testing.T) {
	wasInTestEnvironment := retry.InTestEnvironment
	retry.InTestEnvironment = true
	t.Cleanup(func() { retry.InTestEnvironment = wasInTestEnvironment })

	tests := []struct {
		name            string
		master2Ready    corev1.ConditionStatus
		secretEndpoints string
		expectedErrs    []string
		expectedConfig  string
	}{
		{
			name:            "ready survivors",
			master2Ready:    corev1.ConditionTrue,
			secretEndpoints: bothSurvivorsEndpoints,
			expectedConfig:  base64.StdEncoding.EncodeToString([]byte("#cloud-config")),
		},
		{
			name:            "not ready survivor",
			master2Ready:    corev1.ConditionFalse,
			secretEndpoints: bothSurvivorsEndpoints,
			expectedErrs:    []string{`get ready API server hosts before replacing master node "cluster-master-0": `, "cluster-master-2"},
			expectedConfig:  "old cloud config",
		},
		{
			// The ready survivors are what the refresher waits for in the Secret.
			name:            "secret lacks a ready survivor",
			master2Ready:    corev1.ConditionTrue,
			secretEndpoints: "- 10.0.0.11:6443\n",
			expectedErrs:    []string{`get refreshed cloud config before replacing master node "cluster-master-0": `, "10.0.0.12"},
			expectedConfig:  "old cloud config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller, convergeCtx, _ := newRefresherTestController(t, tt.master2Ready, tt.secretEndpoints)
			metaConfig := &config.MetaConfig{}

			// A non-zero index catches a refresher that ignores the index it is given.
			const nodeIndex = 2
			refresh := controller.makeMasterNodeVariablesRefresher(convergeCtx, metaConfig, "cluster-master-0", nodeIndex, 1, 0)
			require.NotNil(t, refresh)

			variables, err := refresh(t.Context())
			require.Equal(t, tt.expectedConfig, controller.cloudConfig)
			if len(tt.expectedErrs) > 0 {
				for _, expectedErr := range tt.expectedErrs {
					require.ErrorContains(t, err, expectedErr)
				}
				return
			}

			require.NoError(t, err)
			require.Equal(t, metaConfig.NodeGroupConfig("master", nodeIndex, tt.expectedConfig), variables)
		})
	}
}

func TestMasterNodeVariablesRefresherRetries(t *testing.T) {
	wasInTestEnvironment := retry.InTestEnvironment
	retry.InTestEnvironment = false
	t.Cleanup(func() { retry.InTestEnvironment = wasInTestEnvironment })

	t.Run("waits for a survivor to turn ready", func(t *testing.T) {
		controller, convergeCtx, clientset := newRefresherTestController(t, corev1.ConditionFalse, bothSurvivorsEndpoints)

		lists := 0
		clientset.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
			lists++
			if lists == 2 {
				pod := newAPIServerPod("cluster-master-2", "10.0.0.12", corev1.ConditionTrue)
				require.NoError(t, clientset.Tracker().Update(corev1.SchemeGroupVersion.WithResource("pods"), pod, pod.Namespace))
			}
			return false, nil, nil
		})

		refresh := controller.makeMasterNodeVariablesRefresher(convergeCtx, &config.MetaConfig{}, "cluster-master-0", 0, 3, time.Millisecond)
		_, err := refresh(t.Context())
		require.NoError(t, err)
		require.Equal(t, 2, lists)
		require.Equal(t, base64.StdEncoding.EncodeToString([]byte("#cloud-config")), controller.cloudConfig)
	})

	t.Run("stops on a permanent auth error", func(t *testing.T) {
		controller, convergeCtx, clientset := newRefresherTestController(t, corev1.ConditionTrue, bothSurvivorsEndpoints)

		lists := 0
		clientset.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
			lists++
			return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "", nil)
		})

		refresh := controller.makeMasterNodeVariablesRefresher(convergeCtx, &config.MetaConfig{}, "cluster-master-0", 0, 3, time.Millisecond)
		_, err := refresh(kubeerrors.WithAuthMode(t.Context(), kubeerrors.AuthModeOwnCredentials))
		require.True(t, apierrors.IsForbidden(err), "unexpected error: %v", err)
		require.Equal(t, 1, lists)
		require.Equal(t, "old cloud config", controller.cloudConfig)
	})
}
