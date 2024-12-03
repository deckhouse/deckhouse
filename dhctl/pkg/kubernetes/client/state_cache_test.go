// Copyright 2021 Flant JSC
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

package client

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	v1_type "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/tests"
)

func TestDeckhouseInstall(t *testing.T) {
	log.InitLogger("json")

	t.Run("creates secret when initialize", func(t *testing.T) {
		fakeClient := NewFakeKubernetesClient()

		namespace := "test-ns"
		name := "tst-state"

		additionalLabels := map[string]string{
			"additional": "label",
		}

		k8sCache := NewK8sStateCache(fakeClient, namespace, name, "/tmp/dhctl_tst").
			WithLabels(additionalLabels)

		err := k8sCache.Init()
		require.NoError(t, err)

		secret, err := fakeClient.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		require.NoError(t, err)

		require.Equal(t, secret.Name, name)
		require.Equal(t, secret.Data, map[string][]byte{})

		for k, v := range additionalLabels {
			require.Contains(t, secret.Labels, k)
			require.Equal(t, secret.Labels[k], v)
		}
	})

	t.Run("does not errors initialize when secret already exists", func(t *testing.T) {
		fakeClient := NewFakeKubernetesClient()

		namespace := "test-ns"
		name := "tst-state"

		secretToCreate := &v1_type.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels: map[string]string{
					labelKey("cluster-name"): name,
					labelKey("state"):        "true",
				},
			},

			Data: map[string][]byte{},
		}

		_, err := fakeClient.CoreV1().Secrets(namespace).Create(context.TODO(), secretToCreate, metav1.CreateOptions{})
		require.NoError(t, err)

		k8sCache := NewK8sStateCache(fakeClient, namespace, name, "/tmp/dhctl_tst")
		err = k8sCache.Init()
		require.NoError(t, err)

		list, err := fakeClient.CoreV1().Secrets(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelKey("state")})
		require.NoError(t, err)
		require.Len(t, list.Items, 1)
	})

	t.Run("passes all general cache tests", func(t *testing.T) {
		fakeClient := NewFakeKubernetesClient()

		cacheState := NewK8sStateCache(fakeClient, "tsts", "state", "/tmp/dhctl_tst")
		err := cacheState.Init()
		require.NoError(t, err)

		tests.RunStateCacheTests(t, cacheState)
	})
}
