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

package registrydata

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"

	libcon "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg"
	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/ssh/clissh"
	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/ssh/gossh"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

func createRegistryStorage(t *testing.T, kubeCl *client.KubernetesClient, addresses ...string) {
	t.Helper()

	replicas := make([]interface{}, 0, len(addresses))
	for _, address := range addresses {
		replicas = append(replicas, map[string]interface{}{"node": "master", "address": address})
	}
	_, err := kubeCl.Dynamic().Resource(registryStorageGVR).Create(t.Context(), &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "deckhouse.io/v1alpha1",
			"kind":       "RegistryStorage",
			"metadata":   map[string]interface{}{"name": registryStorageName},
			"status":     map[string]interface{}{"replicas": replicas},
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
}

// TestTheTunnelGoesWhereTheReplicaReportedServing: during the update to the store's current ports a
// replica serves on the previous port until the module replaces it, and a dhctl of the new version
// has to reach it there.
func TestTheTunnelGoesWhereTheReplicaReportedServing(t *testing.T) {
	t.Run("a replica on the previous pod spec", func(t *testing.T) {
		kubeCl := client.NewFakeKubernetesClient()
		createRegistryStorage(t, kubeCl, "10.0.0.2:4281", "10.0.0.1:5001")

		require.Equal(t, registry_const.LegacyStoragePort, servingPort(t.Context(), kubeCl, "10.0.0.1"))
		require.Equal(t, registry_const.StoragePort, servingPort(t.Context(), kubeCl, "10.0.0.2"))
	})

	t.Run("IPv6 in either spelling", func(t *testing.T) {
		kubeCl := client.NewFakeKubernetesClient()
		createRegistryStorage(t, kubeCl, "[fd00:0::1]:5001")

		require.Equal(t, registry_const.LegacyStoragePort, servingPort(t.Context(), kubeCl, "fd00::1"))
		require.Equal(t, registry_const.LegacyStoragePort, servingPort(t.Context(), kubeCl, "[fd00::1]"))
	})

	t.Run("no report, an unreadable one, or none for this host: the current port", func(t *testing.T) {
		kubeCl := client.NewFakeKubernetesClient()
		require.Equal(t, registry_const.StoragePort, servingPort(t.Context(), kubeCl, "10.0.0.1"))

		createRegistryStorage(t, kubeCl, "10.0.0.1", "10.0.0.2:0", "10.0.0.3:5001")
		require.Equal(t, registry_const.StoragePort, servingPort(t.Context(), kubeCl, "10.0.0.1"))
		require.Equal(t, registry_const.StoragePort, servingPort(t.Context(), kubeCl, "10.0.0.2"))
		require.Equal(t, registry_const.StoragePort, servingPort(t.Context(), kubeCl, "10.0.0.30"))
	})
}

type fakeTunnel struct{ libcon.Tunnel }

func (fakeTunnel) Up(context.Context) error { return nil }

// fakeSSH is a Go SSH transport that records the forwards asked of it.
type fakeSSH struct {
	libcon.SSHClient

	forwards []string
}

func (f *fakeSSH) Tunnel(address string) libcon.Tunnel {
	f.forwards = append(f.forwards, address)
	return fakeTunnel{}
}

const (
	testStoreCA       = "-----BEGIN CERTIFICATE-----\nfake-registry-storage-ca\n-----END CERTIFICATE-----\n"
	testStoreUser     = "registry-ro"
	testStorePassword = "dh-test-secret-0123456789abcdef"
)

func createStoreSecret(t *testing.T, kubeCl *client.KubernetesClient, addresses string) {
	t.Helper()

	_, err := kubeCl.CoreV1().Secrets(storeSecretNamespace).Create(t.Context(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: storeSecretName, Namespace: storeSecretNamespace},
		Data: map[string][]byte{
			"addresses": []byte(addresses),
			"ca.crt":    []byte(testStoreCA),
			"username":  []byte(testStoreUser),
			"password":  []byte(testStorePassword),
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
}

func forgetTunnels(t *testing.T) {
	t.Helper()

	tunnels.Lock()
	tunnels.ports = map[string]int{}
	tunnels.Unlock()
	t.Cleanup(func() {
		tunnels.Lock()
		tunnels.ports = map[string]int{}
		tunnels.Unlock()
	})
}

// connectCounting yields sshCl and counts how many times the connection was asked for.
func connectCounting(sshCl libcon.SSHClient, err error) (func(context.Context) (libcon.SSHClient, error), *int) {
	calls := new(int)
	return func(context.Context) (libcon.SSHClient, error) {
		*calls++
		return sshCl, err
	}, calls
}

// TestTheStoreIsReachedThroughTheNode: the in-cluster address of an air-gapped cluster is reached at the
// store, over a forward on the SSH connection, with the store's authority and account.
func TestTheStoreIsReachedThroughTheNode(t *testing.T) {
	forgetTunnels(t)

	kubeCl := client.NewFakeKubernetesClient()
	createStoreSecret(t, kubeCl, "10.0.0.1,10.0.0.2")
	sshCl := &fakeSSH{}
	connect, calls := connectCounting(sshCl, nil)
	kubeCl.WithSSHConnect(connect)

	conf, ok, err := StoreThroughNode(t.Context(), kubeCl, registry_const.HostWithPath+"/modules")
	require.NoError(t, err)
	require.True(t, ok)

	host, path, found := strings.Cut(conf.GetRegistry(), "/")
	require.True(t, found)
	require.Equal(t, "system/deckhouse/modules", path)
	localHost, localPort, err := net.SplitHostPort(host)
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1", localHost)

	require.Equal(t, schemeHTTPS, conf.GetScheme())
	require.Equal(t, testStoreCA, conf.GetCA())
	require.Equal(t, testStoreUser, conf.GetUsername())
	require.Equal(t, testStorePassword, conf.GetPassword())

	require.Equal(t, []string{fmt.Sprintf("10.0.0.1:%d:127.0.0.1:%s", registry_const.StoragePort, localPort)}, sshCl.forwards)
	require.Equal(t, 1, *calls)

	again, ok, err := StoreThroughNode(t.Context(), kubeCl, registry_const.HostWithPath)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, host+"/system/deckhouse", again.GetRegistry())
	require.Len(t, sshCl.forwards, 1, "the forward already open is reused")
}

// TestTheStoreIsReachedOnlyForTheInClusterAddress: any other address is one somebody chose because it
// is reachable, and no SSH connection is opened unless the store is there to reach.
func TestTheStoreIsReachedOnlyForTheInClusterAddress(t *testing.T) {
	forgetTunnels(t)

	t.Run("another address", func(t *testing.T) {
		kubeCl := client.NewFakeKubernetesClient()
		createStoreSecret(t, kubeCl, "10.0.0.1")
		connect, calls := connectCounting(&fakeSSH{}, nil)
		kubeCl.WithSSHConnect(connect)

		_, ok, err := StoreThroughNode(t.Context(), kubeCl, "registry.example.io/deckhouse/ee/modules")
		require.NoError(t, err)
		require.False(t, ok)
		require.Zero(t, *calls)
	})

	t.Run("a cluster that keeps no store", func(t *testing.T) {
		kubeCl := client.NewFakeKubernetesClient()
		connect, calls := connectCounting(&fakeSSH{}, nil)
		kubeCl.WithSSHConnect(connect)

		_, ok, err := StoreThroughNode(t.Context(), kubeCl, registry_const.HostWithPath+"/modules")
		require.NoError(t, err)
		require.False(t, ok)
		require.Zero(t, *calls)
	})

	t.Run("no SSH at all", func(t *testing.T) {
		kubeCl := client.NewFakeKubernetesClient()
		createStoreSecret(t, kubeCl, "10.0.0.1")

		_, ok, err := StoreThroughNode(t.Context(), kubeCl, registry_const.HostWithPath+"/modules")
		require.NoError(t, err)
		require.False(t, ok)
	})

	t.Run("an SSH host that cannot be reached is reported", func(t *testing.T) {
		kubeCl := client.NewFakeKubernetesClient()
		createStoreSecret(t, kubeCl, "10.0.0.1")
		connect, _ := connectCounting(nil, errors.New("ssh: handshake failed"))
		kubeCl.WithSSHConnect(connect)

		_, _, err := StoreThroughNode(t.Context(), kubeCl, registry_const.HostWithPath+"/modules")
		require.ErrorContains(t, err, "ssh: handshake failed")
	})
}

// TestTheForwardIsWrittenForTheBackend: Go SSH and OpenSSH read the forward in opposite orders, and
// either one written the other's way listens on the store's address and dials a port nobody serves.
func TestTheForwardIsWrittenForTheBackend(t *testing.T) {
	require.Equal(t, "10.0.0.1:4281:127.0.0.1:40000", forwardSpec(false, "10.0.0.1", 4281, 40000))
	require.Equal(t, "127.0.0.1:40000:10.0.0.1:4281", forwardSpec(true, "10.0.0.1", 4281, 40000))

	require.True(t, isLegacySSH(&clissh.Client{}))
	require.False(t, isLegacySSH(&gossh.Client{}))
	require.False(t, isLegacySSH(&fakeSSH{}))
}
