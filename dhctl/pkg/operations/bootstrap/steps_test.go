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

package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

func TestNewRegistryClientConfigGetter(t *testing.T) {
	t.Run("Path with leading slash", func(t *testing.T) {
		config := config.RegistryData{
			Address:   "registry.deckhouse.io",
			Path:      "/deckhouse/ee",
			DockerCfg: "eyJhdXRocyI6IHsgInJlZ2lzdHJ5LmRlY2tob3VzZS5pbyI6IHt9fX0=", // {"auths": { "registry.deckhouse.io": {}}}
		}
		getter, err := newRegistryClientConfigGetter(config)
		require.NoError(t, err)
		require.Equal(t, getter.Repository, "registry.deckhouse.io/deckhouse/ee")
	})
	t.Run("Path without leading slash", func(t *testing.T) {
		config := config.RegistryData{
			Address:   "registry.deckhouse.io",
			Path:      "deckhouse/ee",
			DockerCfg: "eyJhdXRocyI6IHsgInJlZ2lzdHJ5LmRlY2tob3VzZS5pbyI6IHt9fX0=", // {"auths": { "registry.deckhouse.io": {}}}
		}
		getter, err := newRegistryClientConfigGetter(config)
		require.NoError(t, err)
		require.Equal(t, getter.Repository, "registry.deckhouse.io/deckhouse/ee")
	})
	t.Run("Host with port, path with leading slash", func(t *testing.T) {
		config := config.RegistryData{
			Address:   "registry.deckhouse.io:30000",
			Path:      "/deckhouse/ee",
			DockerCfg: "eyJhdXRocyI6IHsgInJlZ2lzdHJ5LmRlY2tob3VzZS5pbzozMDAwMCI6IHt9fX0=", // {"auths": { "registry.deckhouse.io:30000": {}}}
		}
		getter, err := newRegistryClientConfigGetter(config)
		require.NoError(t, err)
		require.Equal(t, getter.Repository, "registry.deckhouse.io:30000/deckhouse/ee")
	})
	t.Run("Host with port, path without leading slash", func(t *testing.T) {
		config := config.RegistryData{
			Address:   "registry.deckhouse.io:30000",
			Path:      "deckhouse/ee",
			DockerCfg: "eyJhdXRocyI6IHsgInJlZ2lzdHJ5LmRlY2tob3VzZS5pbzozMDAwMCI6IHt9fX0=", // {"auths": { "registry.deckhouse.io:30000	": {}}}
		}
		getter, err := newRegistryClientConfigGetter(config)
		require.NoError(t, err)
		require.Equal(t, getter.Repository, "registry.deckhouse.io:30000/deckhouse/ee")
	})
}

func TestBootstrapGetNodesFromCache(t *testing.T) {
	log.InitLogger("simple")
	dir, err := os.MkdirTemp(os.TempDir(), "dhctl-test-bootstrap-*")
	defer os.RemoveAll(dir)

	require.NoError(t, err)

	for _, name := range []string{
		"base-infrastructure.tfstate",
		"some_trash",
		"test-master-0.tfstate",
		"test-master-1.tfstate",
		"test-master-without-index.tfstate",
		"test-master-1.tfstate.backup",
		"uuid.tfstate",
		"test-static-ingress-0.tfstate",
	} {
		_, err := os.Create(filepath.Join(dir, name))
		require.NoError(t, err)
	}

	t.Run("Should get only nodes state from cache", func(t *testing.T) {
		stateCache, err := cache.NewStateCache(dir)
		require.NoError(t, err)

		result, err := BootstrapGetNodesFromCache(&config.MetaConfig{ClusterPrefix: "test"}, stateCache)
		require.NoError(t, err)

		require.Len(t, result["master"], 2)
		require.Len(t, result["static-ingress"], 1)

		require.Equal(t, "test-master-0", result["master"][0])
		require.Equal(t, "test-master-1", result["master"][1])

		require.Equal(t, "test-static-ingress-0", result["static-ingress"][0])
	})
}

func TestInstallDeckhouse(t *testing.T) {
	createReadyDeckhousePod := func(fakeClient *client.KubernetesClient) {
		pod := &v1.Pod{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Pod",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "deckhouse-pod",
				Namespace: "d8-system",
				Labels: map[string]string{
					"app": "deckhouse",
				},
			},
			Status: v1.PodStatus{
				Phase: v1.PodRunning,
				Conditions: []v1.PodCondition{
					{
						Type:   v1.PodReady,
						Status: v1.ConditionTrue,
					},
				},
			},
		}

		_, err := fakeClient.CoreV1().Pods("d8-system").Create(context.TODO(), pod, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
	}

	createUUIDConfigMap := func(fakeClient *client.KubernetesClient, uuid string) {
		cm := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      manifests.ClusterUUIDCm,
				Namespace: manifests.ClusterUUIDCmNamespace,
			},
			Data: map[string]string{manifests.ClusterUUIDCmKey: uuid},
		}

		_, err := fakeClient.CoreV1().ConfigMaps(manifests.ClusterUUIDCmNamespace).Create(context.TODO(), cm, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
	}

	clusterUUID := "848c3b2c-eda6-11ec-9289-dff550c719eb"

	conf := &config.DeckhouseInstaller{
		Bundle:    "minimal",
		LogLevel:  "Info",
		UUID:      clusterUUID,
		DevBranch: "pr1111",
	}

	assertDeploymentAndUUIDCmCreated := func(t *testing.T, fakeClient *client.KubernetesClient) {
		// todo assert all manifests
		_, err := fakeClient.AppsV1().Deployments("d8-system").Get(context.TODO(), "deckhouse", metav1.GetOptions{})
		require.NoError(t, err)

		uuidCm, err := fakeClient.CoreV1().ConfigMaps(manifests.ClusterUUIDCmNamespace).Get(context.TODO(), manifests.ClusterUUIDCm, metav1.GetOptions{})
		require.NoError(t, err)

		require.Equal(t, uuidCm.Data[manifests.ClusterUUIDCmKey], clusterUUID)
	}

	assertNotDeploymentCreatedAndUUIDCmIsSame := func(t *testing.T, fakeClient *client.KubernetesClient, uuid string) {
		// todo assert all manifests
		_, err := fakeClient.AppsV1().Deployments("d8-system").Get(context.TODO(), "deckhouse", metav1.GetOptions{})
		require.NotNil(t, err)

		uuidCm, err := fakeClient.CoreV1().ConfigMaps(manifests.ClusterUUIDCmNamespace).Get(context.TODO(), manifests.ClusterUUIDCm, metav1.GetOptions{})
		require.NoError(t, err)

		require.Equal(t, uuidCm.Data[manifests.ClusterUUIDCmKey], uuid)
	}

	t.Run("Does not have cluster uuid config map", func(t *testing.T) {
		t.Run("should install Deckhouse", func(t *testing.T) {
			fakeClient := client.NewFakeKubernetesClient()
			createReadyDeckhousePod(fakeClient)

			err := InstallDeckhouse(fakeClient, conf)

			require.NoError(t, err, "Should install Deckhouse")

			assertDeploymentAndUUIDCmCreated(t, fakeClient)
		})
	})

	t.Run("Cluster has uuid config map", func(t *testing.T) {
		t.Run("with empty uuid", func(t *testing.T) {
			t.Run("should not install Deckhouse", func(t *testing.T) {
				fakeClient := client.NewFakeKubernetesClient()
				curUUID := ""

				createReadyDeckhousePod(fakeClient)
				createUUIDConfigMap(fakeClient, curUUID)

				err := InstallDeckhouse(fakeClient, conf)

				require.Error(t, err, "Should not install Deckhouse")

				assertNotDeploymentCreatedAndUUIDCmIsSame(t, fakeClient, curUUID)
			})
		})

		t.Run("with another uuid", func(t *testing.T) {
			t.Run("should not install Deckhouse", func(t *testing.T) {
				fakeClient := client.NewFakeKubernetesClient()

				curUUID := uuid.New().String()

				createReadyDeckhousePod(fakeClient)
				createUUIDConfigMap(fakeClient, curUUID)

				err := InstallDeckhouse(fakeClient, conf)

				require.Error(t, err, "Should not install Deckhouse")

				assertNotDeploymentCreatedAndUUIDCmIsSame(t, fakeClient, curUUID)
			})
		})

		t.Run("with same uuid", func(t *testing.T) {
			t.Run("should install deckhouse", func(t *testing.T) {
				fakeClient := client.NewFakeKubernetesClient()
				createReadyDeckhousePod(fakeClient)
				createUUIDConfigMap(fakeClient, clusterUUID)

				err := InstallDeckhouse(fakeClient, conf)

				require.NoError(t, err, "Should install Deckhouse")

				assertDeploymentAndUUIDCmCreated(t, fakeClient)
			})
		})
	})
}
