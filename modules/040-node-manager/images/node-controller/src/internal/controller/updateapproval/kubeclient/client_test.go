/*
Copyright 2025 Flant JSC

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

package kubeclient

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ua "github.com/deckhouse/node-controller/internal/controller/updateapproval/common"
)

func newChecksumSecret(checksums map[string]string) *corev1.Secret {
	data := make(map[string][]byte)
	for k, v := range checksums {
		data[k] = []byte(v)
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ua.ConfigurationChecksumsSecretName,
			Namespace: ua.MachineNamespace,
		},
		Data: data,
	}
}

func TestGetConfigurationChecksums(t *testing.T) {
	t.Run("returns checksums from secret", func(t *testing.T) {
		secret := newChecksumSecret(map[string]string{"worker": "cs1", "master": "cs2"})
		scheme := runtime.NewScheme()
		_ = corev1.AddToScheme(scheme)
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

		checksums, err := Client{Client: c}.GetConfigurationChecksums(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "cs1", checksums["worker"])
		assert.Equal(t, "cs2", checksums["master"])
	})

	t.Run("returns nil when secret not found", func(t *testing.T) {
		scheme := runtime.NewScheme()
		_ = corev1.AddToScheme(scheme)
		c := fake.NewClientBuilder().WithScheme(scheme).Build()

		checksums, err := Client{Client: c}.GetConfigurationChecksums(context.Background())
		require.NoError(t, err)
		assert.Nil(t, checksums)
	})
}
