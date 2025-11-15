// Copyright 2023 Flant JSC
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

package deckhouse

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
)

// ConvergeDeckhouseConfigurationForCommander – reconciles deckhouse in-cluster configmaps and secrets.
// This function used in commander-mode, which stores primary configuration in the storage outside of cluster,
// and periodically reconciles configuration inside cluster to match configuration stored outside of cluster.
func ConvergeDeckhouseConfigurationForCommander(ctx context.Context, kubeCl *client.KubernetesClient, commanderUUID uuid.UUID, metaConfig *config.MetaConfig) error {
	tasks, err := getTasksForRunning(ctx, kubeCl, commanderUUID, metaConfig)
	if err != nil {
		return err
	}

	return log.Process("default", "Converge deckhouse configuration", func() error {
		for _, task := range tasks {
			err := task.CreateOrUpdate()
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func getTasksForRunning(ctx context.Context, kubeCl *client.KubernetesClient, commanderUUID uuid.UUID, metaConfig *config.MetaConfig) ([]actions.ManifestTask, error) {
	clusterUUID := metaConfig.UUID
	if clusterUUID == "" {
		return nil, fmt.Errorf("Converge deckhouse manifest. Cluster UUID cannot be empty")
	}

	clusterConfig, err := metaConfig.ClusterConfigYAML()
	if err != nil {
		return nil, fmt.Errorf("Unable to get cluster config yaml: %w", err)
	}

	// cluster configuration can be empty for deckhouse in managed clusters
	// but commander does not support it in current time
	// we protect converge with empty configuration to avoid errors in commander
	if len(clusterConfig) == 0 {
		return nil, fmt.Errorf("Cluster configuration is empty. Cannot converge deckhouse manifest because commander does not support managed installations")
	}

	tasks := []actions.ManifestTask{
		{
			Name:     `Secret "d8-cluster-configuration"`,
			Manifest: func() interface{} { return manifests.SecretWithClusterConfig(clusterConfig) },
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("kube-system").Create(ctx, manifest.(*apiv1.Secret), metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("kube-system").Update(ctx, manifest.(*apiv1.Secret), metav1.UpdateOptions{})
				return err
			},
		},
		{
			Name: `ConfigMap "d8-cluster-uuid"`,
			Manifest: func() interface{} {
				return manifests.ClusterUUIDConfigMap(clusterUUID)
			},
			CreateFunc: func(manifest interface{}) error {
				// NOTE: Uuid configmap uses "more careful" update task,
				// NOTE:  which will create configmap only if it does not exist,
				// NOTE:  or update configmap only if actual uuid in configmap does not match target uuid.
				actualManifest, err := kubeCl.CoreV1().ConfigMaps(manifests.ClusterUUIDCmNamespace).Get(ctx, manifest.(*apiv1.ConfigMap).Name, metav1.GetOptions{})
				if errors.IsAlreadyExists(err) {
					if actualManifest.Data[manifests.ClusterUUIDCmKey] != manifest.(*apiv1.ConfigMap).Data[manifests.ClusterUUIDCmKey] {
						// Update manifest only if update needed
						return err
					} else {
						// Do nothing if manifest is actual
						return nil
					}
				}

				_, err = kubeCl.CoreV1().ConfigMaps(manifests.ClusterUUIDCmNamespace).Create(ctx, manifest.(*apiv1.ConfigMap), metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().ConfigMaps(manifests.ClusterUUIDCmNamespace).Update(ctx, manifest.(*apiv1.ConfigMap), metav1.UpdateOptions{})
				return err
			},
		},
	}

	if commanderUUID != uuid.Nil {
		tasks = append(tasks, commander.ConstructManagedByCommanderConfigMapTask(ctx, commanderUUID, kubeCl))
	}

	switch metaConfig.ClusterType {
	case config.CloudClusterType:
		cloudTask, err := getCloudClusterSettingsTask(ctx, kubeCl, metaConfig)
		if err != nil {
			return nil, err
		}

		tasks = append(tasks, *cloudTask)
	case config.StaticClusterType:
		staticTask, err := getStaticClusterSettingsTask(ctx, kubeCl, metaConfig)
		if err != nil {
			return nil, err
		}

		tasks = append(tasks, *staticTask)
	case "":
		return nil, fmt.Errorf("Cannot converge deckhouse manifest because commander does not support managed installations")
	default:
		return nil, fmt.Errorf("Unsupported cluster type: '%s'", metaConfig.ClusterType)
	}

	return tasks, nil
}

func getCloudClusterSettingsTask(ctx context.Context, kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig) (*actions.ManifestTask, error) {
	providerClusterConfig, err := metaConfig.ProviderClusterConfigYAML()
	if err != nil {
		return nil, fmt.Errorf("Unable to get provider cluster config yaml from MetaConfig: %w", err)
	}

	if len(providerClusterConfig) == 0 {
		return nil, fmt.Errorf("ProviderClusterConfiguration section is required for a Cloud cluster.")
	}

	const secretName = "d8-provider-cluster-configuration"

	return &actions.ManifestTask{
		Name: fmt.Sprintf(`Secret "%s"`, secretName),
		Manifest: func() interface{} {
			return manifests.SecretWithProviderClusterConfig(
				providerClusterConfig, nil,
			)
		},
		CreateFunc: func(manifest interface{}) error {
			return convergeManifestsCreateSecret(ctx, kubeCl, manifest, secretName)
		},
		UpdateFunc: func(manifest interface{}) error {
			return convergeManifestsPatchSecret(ctx, kubeCl, manifest, secretName)
		},
	}, nil
}

func getStaticClusterSettingsTask(ctx context.Context, kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig) (*actions.ManifestTask, error) {
	staticClusterConfig, err := metaConfig.StaticClusterConfigYAML()
	if err != nil {
		return nil, fmt.Errorf("Unable to get static cluster config: %w", err)
	}

	if len(staticClusterConfig) == 0 {
		// static cluster configuration can be empty because we have auto discovering interfaces
		log.DebugLn("No static cluster configuration section found. Rewrite with empty data because we have auto discovery")
	}

	const secretName = "d8-static-cluster-configuration"

	return &actions.ManifestTask{
		Name: fmt.Sprintf(`Secret "%s"`, secretName),
		Manifest: func() interface{} {
			return manifests.SecretWithStaticClusterConfig(staticClusterConfig)
		},
		CreateFunc: func(manifest interface{}) error {
			return convergeManifestsCreateSecret(ctx, kubeCl, manifest, secretName)
		},
		UpdateFunc: func(manifest interface{}) error {
			return convergeManifestsPatchSecret(ctx, kubeCl, manifest, secretName)
		},
	}, nil
}

func convergeManifestsCreateSecret(ctx context.Context, kubeCl *client.KubernetesClient, manifest any, secretType string) error {
	secret, ok := manifest.(*apiv1.Secret)
	if !ok {
		return fmt.Errorf("Cannot cast %s secret", secretType)
	}

	_, err := kubeCl.CoreV1().Secrets(secret.GetNamespace()).Create(ctx, secret, metav1.CreateOptions{})

	return err
}

func convergeManifestsPatchSecret(ctx context.Context, kubeCl *client.KubernetesClient, manifest any, secretType string) error {
	secret, ok := manifest.(*apiv1.Secret)
	if !ok {
		return fmt.Errorf("Cannot cast %s secret", secretType)
	}

	data, err := json.Marshal(secret)
	if err != nil {
		return fmt.Errorf("Cannot marshal %s secret: %w", secretType, err)
	}

	_, err = kubeCl.CoreV1().Secrets(secret.GetNamespace()).Patch(ctx,
		secret.GetName(),
		types.MergePatchType,
		data,
		metav1.PatchOptions{},
	)

	return err
}
