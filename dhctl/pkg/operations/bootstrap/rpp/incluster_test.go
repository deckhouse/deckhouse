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

package rpp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

func proxyDeployment(replicas, ready int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: inClusterProxyNamespace,
			Name:      inClusterProxyDeployment,
		},
		Status: appsv1.DeploymentStatus{
			Replicas:      replicas,
			ReadyReplicas: ready,
		},
	}
}

func TestWaitForInClusterProxyReady(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()
	_, err := kubeCl.AppsV1().Deployments(inClusterProxyNamespace).
		Create(t.Context(), proxyDeployment(1, 1), metav1.CreateOptions{})
	require.NoError(t, err)

	require.NoError(t, waitForInClusterProxy(t.Context(), kubeCl.KubeClient, 1, time.Millisecond))
}

// An HA Deployment asks for a replica per master, and dhctl has built only the first one when
// this wait runs: the machines that bring the rest up are exactly what it guards.
func TestWaitForInClusterProxyOneOfManyReplicasIsEnough(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()
	_, err := kubeCl.AppsV1().Deployments(inClusterProxyNamespace).
		Create(t.Context(), proxyDeployment(3, 1), metav1.CreateOptions{})
	require.NoError(t, err)

	require.NoError(t, waitForInClusterProxy(t.Context(), kubeCl.KubeClient, 1, time.Millisecond))
}

func TestWaitForInClusterProxyNoReadyReplica(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()
	_, err := kubeCl.AppsV1().Deployments(inClusterProxyNamespace).
		Create(t.Context(), proxyDeployment(1, 0), metav1.CreateOptions{})
	require.NoError(t, err)

	err = waitForInClusterProxy(t.Context(), kubeCl.KubeClient, 2, time.Millisecond)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no ready replica")
}

// The module has not rolled out at all: the wait keeps polling rather than treating the missing
// Deployment as a permanent failure, since it appears only once Deckhouse converges.
func TestWaitForInClusterProxyDeploymentMissing(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()

	err := waitForInClusterProxy(t.Context(), kubeCl.KubeClient, 2, time.Millisecond)
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not exist yet")
}
