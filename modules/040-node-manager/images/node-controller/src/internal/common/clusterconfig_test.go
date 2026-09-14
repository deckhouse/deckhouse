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

package common

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const testClusterConfigYAML = "podSubnetCIDR: 10.111.0.0/16\nserviceSubnetCIDR: 10.222.0.0/16\nclusterDomain: cluster.local\ncloud:\n  prefix: prod\n"

func TestReadClusterConfiguration(t *testing.T) {
	want := ClusterConfiguration{
		PodSubnetCIDR:     "10.111.0.0/16",
		ServiceSubnetCIDR: "10.222.0.0/16",
		ClusterDomain:     "cluster.local",
	}
	want.Cloud.Prefix = "prod"

	for _, testCase := range []struct {
		name  string
		value string
	}{
		{name: "raw yaml", value: testClusterConfigYAML},
		{name: "base64 yaml", value: base64.StdEncoding.EncodeToString([]byte(testClusterConfigYAML))},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			client := newClusterConfigTestClient(t, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: ClusterConfigSecretName, Namespace: ClusterConfigSecretNamespace},
				Data:       map[string][]byte{clusterConfigSecretKey: []byte(testCase.value)},
			}).Build()

			got, err := ReadClusterConfiguration(t.Context(), client)
			require.NoError(t, err)
			require.Equal(t, want, got)
		})
	}

	t.Run("missing secret", func(t *testing.T) {
		_, err := ReadClusterConfiguration(t.Context(), newClusterConfigTestClient(t).Build())
		require.Error(t, err)
	})

	t.Run("missing key", func(t *testing.T) {
		client := newClusterConfigTestClient(t, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: ClusterConfigSecretName, Namespace: ClusterConfigSecretNamespace},
		}).Build()
		_, err := ReadClusterConfiguration(t.Context(), client)
		require.ErrorContains(t, err, clusterConfigSecretKey)
	})
}

func newClusterConfigTestClient(t *testing.T, objects ...runtime.Object) *fake.ClientBuilder {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	return fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...)
}
