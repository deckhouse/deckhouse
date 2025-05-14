// Copyright 2025 Flant JSC
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

package infrastructure

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

func yamlToSecret(content string) *corev1.Secret {
	var secret corev1.Secret
	err := yaml.Unmarshal([]byte(content), &secret)
	if err != nil {
		panic(err)
	}
	return &secret
}

const (
	cluster = `
apiVersion: v1
data:
  cluster-tf-state.json: c2VjcmV0
kind: Secret
metadata:
  labels:
    heritage: deckhouse
  name: d8-cluster-terraform-state
  namespace: d8-system
type: Opaque
`
	master = `
apiVersion: v1
data:
  node-tf-state.json: c2VjcmV0
kind: Secret
metadata:
  labels:
    heritage: deckhouse
    node.deckhouse.io/node-group: master
    node.deckhouse.io/node-name: nmit-delete-12-03-master-0
    node.deckhouse.io/terraform-state: ""
  name: d8-node-terraform-state-nmit-delete-12-03-master-0
  namespace: d8-system
type: Opaque
`
	node = `
apiVersion: v1
data:
  node-group-settings.json: c2VjcmV0
  node-tf-state.json: c2VjcmV0
kind: Secret
metadata:
  labels:
    heritage: deckhouse
    node.deckhouse.io/node-group: khm
    node.deckhouse.io/node-name: nmit-delete-12-03-khm-0
    node.deckhouse.io/terraform-state: ""
  name: d8-node-terraform-state-nmit-delete-12-03-khm-0
  namespace: d8-system
type: Opaque`

	backupCluster = `
apiVersion: v1
data:
  cluster-tf-state.json: c2VjcmV0
kind: Secret
metadata:
  annotations:
    dhctl.deckhouse.io/before-tofu-state-backup-time: "2025-04-01T23:02:13+03:00"
  labels:
    heritage: deckhouse
    dhctl.deckhouse.io/before-tofu-state-backup: "true"
  name: tf-bkp-cluster-state
  namespace: d8-system
type: Opaque
`
	backupMaster = `
apiVersion: v1
data:
  node-tf-state.json: c2VjcmV0
kind: Secret
metadata:
  annotations:
    dhctl.deckhouse.io/before-tofu-state-backup-time: "2025-04-01T23:02:13+03:00"
  labels:
    heritage: deckhouse
    node.deckhouse.io/node-group: master
    node.deckhouse.io/node-name: nmit-delete-12-03-master-0
    node.deckhouse.io/terraform-state: ""
    dhctl.deckhouse.io/before-tofu-state-backup: "true"
  name: tf-bkp-node-nmit-delete-12-03-master-0
  namespace: d8-system
type: Opaque
`
	backupNode = `
apiVersion: v1
data:
  node-group-settings.json: c2VjcmV0
  node-tf-state.json: c2VjcmV0
kind: Secret
metadata:
  annotations:
    dhctl.deckhouse.io/before-tofu-state-backup-time: "2025-04-01T23:02:13+03:00"
  labels:
    heritage: deckhouse
    node.deckhouse.io/node-group: khm
    node.deckhouse.io/node-name: nmit-delete-12-03-khm-0
    node.deckhouse.io/terraform-state: ""
    dhctl.deckhouse.io/before-tofu-state-backup: "true"
  name: tf-bkp-node-nmit-delete-12-03-khm-0
  namespace: d8-system
type: Opaque`
)

func createSecret(t *testing.T, fakeClient *client.KubernetesClient, content string) *corev1.Secret {
	s, err := fakeClient.CoreV1().Secrets("d8-system").Create(context.TODO(), yamlToSecret(content), metav1.CreateOptions{})

	require.NoError(t, err)
	require.NotNil(t, s)

	return s
}

func assertSecretDidNotChanged(t *testing.T, fakeClient *client.KubernetesClient, secret *corev1.Secret, name string) {
	afterBackupSecret, err := fakeClient.CoreV1().Secrets("d8-system").Get(context.TODO(), name, metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, secret, afterBackupSecret)
}

func getBackupSecret(t *testing.T, fakeClient *client.KubernetesClient, name string) *corev1.Secret {
	backupSecret, err := fakeClient.CoreV1().Secrets("d8-system").Get(context.TODO(), name, metav1.GetOptions{})
	require.NoError(t, err)

	return backupSecret
}

func assertBackupSecretContainsLabelAndAnnotation(t *testing.T, secret *corev1.Secret) {
	require.Equal(t, secret.Labels["dhctl.deckhouse.io/before-tofu-state-backup"], "true")
	require.Equal(t, secret.Labels["dhctl.deckhouse.io/state-backup"], "true")
	require.Contains(t, secret.Annotations, "dhctl.deckhouse.io/before-tofu-state-backup-time")
}

type FakeKubeProvider struct {
	kubeClient *client.KubernetesClient
}

func NewFakeKubeProvider(cl *client.KubernetesClient) *FakeKubeProvider {
	return &FakeKubeProvider{
		kubeClient: cl,
	}
}

func (f *FakeKubeProvider) KubeClient() *client.KubernetesClient {
	return f.kubeClient
}

func TestBackupStates(t *testing.T) {
	fakeClient := client.NewFakeKubernetesClient()
	provider := NewFakeKubeProvider(fakeClient)

	clusterSecret := createSecret(t, fakeClient, cluster)
	masterSecret := createSecret(t, fakeClient, master)
	nodeSecret := createSecret(t, fakeClient, node)

	backuper := NewTofuMigrationStateBackuper(provider, log.GetDefaultLogger())

	err := backuper.BackupStates(context.TODO())
	require.NoError(t, err)

	// check that after backup old secrets did not affect
	assertSecretDidNotChanged(t, fakeClient, clusterSecret, "d8-cluster-terraform-state")
	assertSecretDidNotChanged(t, fakeClient, masterSecret, "d8-node-terraform-state-nmit-delete-12-03-master-0")
	assertSecretDidNotChanged(t, fakeClient, nodeSecret, "d8-node-terraform-state-nmit-delete-12-03-khm-0")

	clusterBackupSecret := getBackupSecret(t, fakeClient, "tf-bkp-cluster-state")
	masterBackupSecret := getBackupSecret(t, fakeClient, "tf-bkp-node-nmit-delete-12-03-master-0")
	nodeBackupSecret := getBackupSecret(t, fakeClient, "tf-bkp-node-nmit-delete-12-03-khm-0")

	assertBackupSecretContainsLabelAndAnnotation(t, clusterBackupSecret)
	assertBackupSecretContainsLabelAndAnnotation(t, masterBackupSecret)
	assertBackupSecretContainsLabelAndAnnotation(t, nodeBackupSecret)

	require.Equal(t, clusterBackupSecret.Data["cluster-tf-state.json"], []byte("secret"))
	require.Equal(t, masterBackupSecret.Data["node-tf-state.json"], []byte("secret"))
	require.Equal(t, nodeBackupSecret.Data["node-tf-state.json"], []byte("secret"))
	require.Equal(t, nodeBackupSecret.Data["node-group-settings.json"], []byte("secret"))
}

func saveCacheState(t *testing.T, c state.Cache, key string, val string) {
	err := c.Save(key, []byte(val))
	require.NoError(t, err)
}

func assertCacheState(t *testing.T, c state.Cache, key string, val string) {
	baseBackup, err := c.Load(key)
	require.NoError(t, err)
	require.Equal(t, baseBackup, []byte(val))
}

func TestBackupStatesForCommander(t *testing.T) {
	fakeClient := client.NewFakeKubernetesClient()
	provider := NewFakeKubeProvider(fakeClient)

	_ = createSecret(t, fakeClient, cluster)
	_ = createSecret(t, fakeClient, master)
	_ = createSecret(t, fakeClient, node)

	c := cache.NewTestCache()

	saveCacheState(t, c, "base-infrastructure.tfstate", "secret")
	saveCacheState(t, c, "fake-nmit-delete-12-03-master-0.tfstate", "secret")
	saveCacheState(t, c, "fake-nmit-delete-12-03-khm-0.tfstate", "secret")

	backuper := NewTofuMigrationStateBackuper(provider, log.GetDefaultLogger()).WithCommanderMode(&TofuBackupCommanderMode{
		Cache:      c,
		MetaConfig: &config.MetaConfig{ClusterPrefix: "fake"},
	})

	err := backuper.BackupStates(context.TODO())
	require.NoError(t, err)

	assertCacheState(t, c, "base-infrastructure.tfstate", "secret")
	assertCacheState(t, c, "fake-nmit-delete-12-03-master-0.tfstate", "secret")
	assertCacheState(t, c, "fake-nmit-delete-12-03-khm-0.tfstate", "secret")

	assertCacheState(t, c, "tf-bkp-cluster-state.terraform.backup", "secret")
	assertCacheState(t, c, "tf-fake-nmit-delete-12-03-master-0.terraform.backup", "secret")
	assertCacheState(t, c, "tf-fake-nmit-delete-12-03-khm-0.terraform.backup", "secret")
}

func TestSkipBackupStatesForCommander(t *testing.T) {
	fakeClient := client.NewFakeKubernetesClient()
	provider := NewFakeKubeProvider(fakeClient)

	_ = createSecret(t, fakeClient, cluster)
	_ = createSecret(t, fakeClient, master)
	_ = createSecret(t, fakeClient, node)

	c := cache.NewTestCache()

	saveCacheState(t, c, "base-infrastructure.tfstate", "secret")
	saveCacheState(t, c, "fake-nmit-delete-12-03-master-0.tfstate", "secret")
	saveCacheState(t, c, "fake-nmit-delete-12-03-khm-0.tfstate", "secret")

	saveCacheState(t, c, "tf-bkp-cluster-state.terraform.backup", "secret1")
	saveCacheState(t, c, "tf-fake-nmit-delete-12-03-master-0.terraform.backup", "secret1")
	saveCacheState(t, c, "tf-fake-nmit-delete-12-03-khm-0.terraform.backup", "secret1")

	backuper := NewTofuMigrationStateBackuper(provider, log.GetDefaultLogger()).WithCommanderMode(&TofuBackupCommanderMode{
		Cache:      c,
		MetaConfig: &config.MetaConfig{ClusterPrefix: "fake"},
	})

	err := backuper.BackupStates(context.TODO())
	require.NoError(t, err)

	assertCacheState(t, c, "base-infrastructure.tfstate", "secret")
	assertCacheState(t, c, "fake-nmit-delete-12-03-master-0.tfstate", "secret")
	assertCacheState(t, c, "fake-nmit-delete-12-03-khm-0.tfstate", "secret")

	assertCacheState(t, c, "tf-bkp-cluster-state.terraform.backup", "secret1")
	assertCacheState(t, c, "tf-fake-nmit-delete-12-03-master-0.terraform.backup", "secret1")
	assertCacheState(t, c, "tf-fake-nmit-delete-12-03-khm-0.terraform.backup", "secret1")

	require.Len(t, c.Store, 6)
}

func TestSkipBackupStatesIfBackupExist(t *testing.T) {
	fakeClient := client.NewFakeKubernetesClient()
	provider := NewFakeKubeProvider(fakeClient)

	clusterSecret := createSecret(t, fakeClient, cluster)
	masterSecret := createSecret(t, fakeClient, master)
	nodeSecret := createSecret(t, fakeClient, node)

	clusterBackupSecret := createSecret(t, fakeClient, backupCluster)
	masterBackupSecret := createSecret(t, fakeClient, backupMaster)
	nodeBackupSecret := createSecret(t, fakeClient, backupNode)

	backuper := NewTofuMigrationStateBackuper(provider, log.GetDefaultLogger())

	err := backuper.BackupStates(context.TODO())
	require.NoError(t, err)

	// check that after backup old secrets did not affect
	assertSecretDidNotChanged(t, fakeClient, clusterSecret, "d8-cluster-terraform-state")
	assertSecretDidNotChanged(t, fakeClient, masterSecret, "d8-node-terraform-state-nmit-delete-12-03-master-0")
	assertSecretDidNotChanged(t, fakeClient, nodeSecret, "d8-node-terraform-state-nmit-delete-12-03-khm-0")

	assertSecretDidNotChanged(t, fakeClient, clusterBackupSecret, "tf-bkp-cluster-state")
	assertSecretDidNotChanged(t, fakeClient, masterBackupSecret, "tf-bkp-node-nmit-delete-12-03-master-0")
	assertSecretDidNotChanged(t, fakeClient, nodeBackupSecret, "tf-bkp-node-nmit-delete-12-03-khm-0")

}
