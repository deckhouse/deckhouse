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

package immutable

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// What the node's handoff endpoint serves: a certificate for kubernetes-admin,
// no key, no impersonation.
const impersonationCollected = `apiVersion: v1
kind: Config
clusters:
- name: kubernetes
  cluster:
    server: https://127.0.0.1:6445
contexts:
- name: kubernetes-admin@kubernetes
  context:
    cluster: kubernetes
    user: kubernetes-admin
current-context: kubernetes-admin@kubernetes
users:
- name: kubernetes-admin
  user:
    client-certificate-data: dGVzdA==
`

func completedForImpersonation(t *testing.T) []byte {
	t.Helper()

	complete, err := withClientKey([]byte(impersonationCollected), "-----BEGIN EC PRIVATE KEY-----\ntest\n-----END EC PRIVATE KEY-----\n")
	require.NoError(t, err)
	return complete
}

func retargetedRESTConfig(t *testing.T) *rest.Config {
	t.Helper()

	out, err := RetargetKubeconfig(t.Context(), completedForImpersonation(t), "https://192.168.1.10:6443", handoffTestNodeName)
	require.NoError(t, err)

	// Built the way every client on this path is built, so what is asserted is
	// what client-go resolved rather than a field the test set itself.
	cfg, err := clientcmd.RESTConfigFromKubeConfig(out)
	require.NoError(t, err)
	return cfg
}

// Deckhouse's admission policies exempt the username "dhctl" and nothing else,
// and the certificate the node signs names kubernetes-admin. Without the group
// the impersonated identity would be a cluster-admin no more: impersonation
// replaces the identity whole, it does not add to it.
func TestTheRetargetedKubeconfigActsAsDhctl(t *testing.T) {
	cfg := retargetedRESTConfig(t)

	require.Equal(t, "dhctl", cfg.Impersonate.UserName)
	require.Equal(t, []string{"system:masters"}, cfg.Impersonate.Groups)
}

// The struct field is not the identity the API server sees. This drives a real
// client-go clientset against a listening server and reads the headers off the
// wire, which is what admission actually decides on.
func TestTheAPIServerIsToldWhoDhctlActsAs(t *testing.T) {
	body, err := json.Marshal(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "d8-system"}})
	require.NoError(t, err)

	// Over a channel rather than a variable: the handler runs on the server's
	// goroutine and only the receive orders its write before the read below.
	seen := make(chan http.Header, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer server.Close()

	cfg := retargetedRESTConfig(t)
	// The fixture's certificate is not a real one and the test server speaks
	// plain HTTP; neither has anything to do with the headers under test.
	cfg.Host = server.URL
	cfg.TLSClientConfig = rest.TLSClientConfig{}

	client, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err)

	_, err = client.CoreV1().Namespaces().Get(t.Context(), "d8-system", metav1.GetOptions{})
	require.NoError(t, err)

	headers := <-seen
	require.Equal(t, "dhctl", headers.Get("Impersonate-User"), "the username every Deckhouse admission policy exempts")
	require.Equal(t, []string{"system:masters"}, headers.Values("Impersonate-Group"), "without it the impersonated user holds nothing")
}

// The operator keeps this copy and reaches the cluster with it by hand. Acting
// as dhctl there would exempt a human from the guardrails those policies are and
// file everything they do under dhctl's name in the audit log.
func TestTheOperatorsKubeconfigDoesNotImpersonate(t *testing.T) {
	kubeconfig, err := clientcmd.Load(completedForImpersonation(t))
	require.NoError(t, err)

	for name, authInfo := range kubeconfig.AuthInfos {
		require.Empty(t, authInfo.Impersonate, "user %s of the operator's copy", name)
		require.Empty(t, authInfo.ImpersonateGroups, "user %s of the operator's copy", name)
	}
}
