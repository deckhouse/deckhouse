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

package immutabletest

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

// The objects are named here in full rather than through the constants the code
// under test uses: renaming one of them is a change of contract with the cluster,
// and these fixtures are what says so.
const (
	// BootstrapTokenSecretName is a secret of the shape the token hook creates.
	BootstrapTokenSecretName = "bootstrap-token-abcdef"

	// BootstrapToken is the token those fixtures publish.
	BootstrapToken = "abcdef.0123456789abcdef" // gitleaks:allow, the shape of a bootstrap token, not one
)

// CreateBootstrapToken publishes the master group's token, one of the three things
// a joining master reads from the cluster.
func CreateBootstrapToken(t *testing.T, kubeCl *client.KubernetesClient) {
	t.Helper()

	_, err := kubeCl.CoreV1().Secrets(global.ConfigsNS).Create(t.Context(), BootstrapTokenSecret(), metav1.CreateOptions{})
	require.NoError(t, err)
}

// BootstrapTokenSecret is the same secret unsaved, for a test that publishes it
// late on purpose.
func BootstrapTokenSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      BootstrapTokenSecretName,
			Namespace: global.ConfigsNS,
			Labels:    map[string]string{"node-manager.deckhouse.io/node-group": global.MasterNodeGroupName},
		},
		Type: corev1.SecretTypeBootstrapToken,
		Data: map[string][]byte{"token-id": []byte("abcdef"), "token-secret": []byte("0123456789abcdef")},
	}
}

// CreateJoinInputsWithoutToken publishes everything a joining master reads from
// the cluster except the bootstrap token.
func CreateJoinInputsWithoutToken(t *testing.T, kubeCl *client.KubernetesClient) {
	t.Helper()

	_, err := kubeCl.CoreV1().ConfigMaps(global.ConfigsNS).Create(t.Context(), &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-root-ca.crt", Namespace: global.ConfigsNS},
		Data:       map[string]string{"ca.crt": "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n"},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	port := int32(6443)
	portName := "https"
	_, err = kubeCl.DiscoveryV1().EndpointSlices("default").Create(t.Context(), &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{Name: "kubernetes", Namespace: "default"},
		Endpoints:  []discoveryv1.Endpoint{{Addresses: []string{"192.168.1.10"}}},
		Ports:      []discoveryv1.EndpointPort{{Name: &portName, Port: &port}},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
}
