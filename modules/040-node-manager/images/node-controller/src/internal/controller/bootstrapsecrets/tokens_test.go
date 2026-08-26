/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package bootstrapsecrets

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

func tokenSecret(name, ng string, validFor time.Duration) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         "kube-system",
			CreationTimestamp: metav1.Now(),
			Labels:            map[string]string{nodecommon.BootstrapTokenNodeGroupLabel: ng},
		},
		Type: corev1.SecretTypeBootstrapToken,
		Data: map[string][]byte{
			"expiration":   []byte(time.Now().Add(validFor).Format(time.RFC3339)),
			"token-id":     []byte("abcdef"),
			"token-secret": []byte("0123456789abcdef"),
		},
	}
}

func newClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

// Порог 3 часа из order_bootstrap_token.go:187: токен, живущий дольше,
// переиспользуется — иначе каждая пересборка секретов плодила бы новый.
func TestEnsureTokenReusesFreshToken(t *testing.T) {
	c := newClient(t, tokenSecret("bootstrap-token-abcdef", "worker", 4*time.Hour))

	token, err := EnsureToken(t.Context(), c, "worker")
	require.NoError(t, err)

	assert.Equal(t, "abcdef.0123456789abcdef", token)

	list := &corev1.SecretList{}
	require.NoError(t, c.List(t.Context(), list))
	assert.Len(t, list.Items, 1, "a fresh token must not be replaced")
}

func TestEnsureTokenMintsWhenExpiringSoon(t *testing.T) {
	c := newClient(t, tokenSecret("bootstrap-token-abcdef", "worker", 2*time.Hour))

	token, err := EnsureToken(t.Context(), c, "worker")
	require.NoError(t, err)

	assert.NotEqual(t, "abcdef.0123456789abcdef", token)

	list := &corev1.SecretList{}
	require.NoError(t, c.List(t.Context(), list))
	assert.Len(t, list.Items, 2, "the old token stays until it expires")
}

func TestEnsureTokenMintsForGroupWithoutToken(t *testing.T) {
	c := newClient(t)

	token, err := EnsureToken(t.Context(), c, "worker")
	require.NoError(t, err)
	assert.Regexp(t, `^[a-z0-9]{6}\.[a-z0-9]{16}$`, token)

	secret := &corev1.Secret{}
	require.NoError(t, c.Get(t.Context(), types.NamespacedName{Namespace: "kube-system", Name: "bootstrap-token-" + token[:6]}, secret))
	assert.Equal(t, corev1.SecretTypeBootstrapToken, secret.Type)
	assert.Equal(t, "worker", secret.Labels[nodecommon.BootstrapTokenNodeGroupLabel])
	assert.Equal(t, []byte("system:bootstrappers:d8-node-manager"), secret.Data["auth-extra-groups"])
	assert.Equal(t, []byte("true"), secret.Data["usage-bootstrap-authentication"])
	assert.Equal(t, []byte("true"), secret.Data["usage-bootstrap-signing"])
}

// Токен именован группой: чужой свежий токен переиспользовать нельзя, иначе
// нода одной группы бутстрапилась бы токеном другой.
func TestEnsureTokenIgnoresOtherGroupsToken(t *testing.T) {
	c := newClient(t, tokenSecret("bootstrap-token-abcdef", "other", 4*time.Hour))

	token, err := EnsureToken(t.Context(), c, "worker")
	require.NoError(t, err)
	assert.NotEqual(t, "abcdef.0123456789abcdef", token)

	secret := &corev1.Secret{}
	require.NoError(t, c.Get(t.Context(), types.NamespacedName{Namespace: "kube-system", Name: "bootstrap-token-" + token[:6]}, secret))
	assert.Equal(t, "worker", secret.Labels[nodecommon.BootstrapTokenNodeGroupLabel])
}

func TestCollectExpiredTokens(t *testing.T) {
	c := newClient(t,
		tokenSecret("bootstrap-token-expired", "worker", -time.Minute),
		tokenSecret("bootstrap-token-alive", "worker", 4*time.Hour),
	)

	require.NoError(t, CollectExpiredTokens(t.Context(), c))

	list := &corev1.SecretList{}
	require.NoError(t, c.List(t.Context(), list))
	require.Len(t, list.Items, 1)
	assert.Equal(t, "bootstrap-token-alive", list.Items[0].Name)
}
