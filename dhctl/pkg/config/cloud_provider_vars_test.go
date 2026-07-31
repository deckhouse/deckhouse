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

package config

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

func TestFetchCredentialSecretsFromCluster_ScopedToProviderNamespace(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()

	// Seed credential Secrets in two different provider namespaces plus an
	// unrelated namespace with the same Secret type. The fetcher must
	// return only the Secrets from the requested provider's namespace.
	for _, ns := range []string{"d8-cloud-provider-aws", "d8-cloud-provider-dvp", "kube-system"} {
		s := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "cloud-credentials", Namespace: ns},
			Type:       CloudProviderCredentialsSecretType,
			Data:       map[string][]byte{"key": []byte(ns)},
		}
		_, err := kubeCl.CoreV1().Secrets(ns).Create(t.Context(), s, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	got, err := fetchCredentialSecretsFromCluster(context.Background(), kubeCl, "dvp")
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Contains(t, got, "d8-cloud-provider-dvp/cloud-credentials")
	require.NotContains(t, got, "d8-cloud-provider-aws/cloud-credentials")
	require.NotContains(t, got, "kube-system/cloud-credentials")
}

func TestFetchCredentialSecretsFromCluster_EmptyProviderReturnsNil(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()
	got, err := fetchCredentialSecretsFromCluster(context.Background(), kubeCl, "")
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestSecretToMap_StringDataForUTF8(t *testing.T) {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: "d8-x"},
		Type:       CloudProviderCredentialsSecretType,
		Data:       map[string][]byte{"token": []byte("hello-world")},
	}

	got := secretToMap(s)
	stringData, ok := got["stringData"].(map[string]string)
	require.True(t, ok)
	require.Equal(t, "hello-world", stringData["token"])
	require.Nil(t, got["data"])
}

func TestSecretToMap_BinaryGoesToDataAsBase64(t *testing.T) {
	raw := []byte{0xff, 0xfe, 0x00, 0x01}
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: "d8-x"},
		Type:       CloudProviderCredentialsSecretType,
		Data:       map[string][]byte{"key": raw},
	}

	got := secretToMap(s)
	data, ok := got["data"].(map[string]string)
	require.True(t, ok)
	require.Equal(t, base64.StdEncoding.EncodeToString(raw), data["key"])
	if sd, ok := got["stringData"].(map[string]string); ok {
		require.NotContains(t, sd, "key")
	}
}
