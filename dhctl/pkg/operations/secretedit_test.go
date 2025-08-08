// Copyright 2024 Flant JSC
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

package operations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

var (
	secretTest          *corev1.Secret
	stateSecretTestYAML = `
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: s2
  namespace: ns1
data:
  cluster-configuration.yaml: cG9kU3VibmV0Tm9kZUNJRFJQcmVmaXg6ICIyNCIKc2VydmljZVN1Ym5ldENJRFI6IDEwLjIyMi4wLjAvMTYK
  cloud-provider-discovery-data.json: eyJ0ZXN0IjogImRhdGEifQ==
`
)

func EditMock(data []byte) ([]byte, error) {
	newData := string(data) + "test: \"25\"\n"
	return []byte(newData), nil
}

func TestSecretEdit(t *testing.T) {
	log.InitLogger("json")

	f := client.NewFakeKubernetesClient()
	retry.InTestEnvironment = true

	_ = yaml.Unmarshal([]byte(stateSecretTestYAML), &secretTest)
	f.KubeClient.CoreV1().Secrets(secretTest.Namespace).Create(context.TODO(), secretTest, metav1.CreateOptions{})

	t.Run("Secret editing", func(t *testing.T) {

		abstractEditing = EditMock
		err := SecretEdit(
			f, "test", secretTest.Namespace, secretTest.Name, "cluster-configuration.yaml",
			map[string]string{"name": "test"},
		)
		require.NoError(t, err)

		secretTestEdit, err := f.KubeClient.CoreV1().Secrets(secretTest.Namespace).Get(context.TODO(), secretTest.Name, metav1.GetOptions{})
		require.NoError(t, err)

		require.Equal(t, "test", secretTestEdit.Labels["name"])

		require.Equal(t, string(secretTestEdit.Data["cloud-provider-discovery-data.json"]), "{\"test\": \"data\"}")

		require.Equal(t, string(secretTestEdit.Data["cluster-configuration.yaml"]), `podSubnetNodeCIDRPrefix: "24"
serviceSubnetCIDR: 10.222.0.0/16
test: "25"
`)
		abstractEditing = Edit
	})
}
