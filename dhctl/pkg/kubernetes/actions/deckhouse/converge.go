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
	"k8s.io/apimachinery/pkg/api/errors"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

// ConvergeDeckhouseConfiguration – reconciles deckhouse in-cluster configmaps and secrets.
// This function used in commander-mode, which stores primary configuration in the storage outside of cluster,
// and periodically reconciles configuration inside cluster to match configuration stored outside of cluster.
func ConvergeDeckhouseConfiguration(ctx context.Context, kubeCl *client.KubernetesClient, clusterUUID string, clusterConfig []byte, providerClusterConfig []byte) error {
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
			Name: `Secret "d8-provider-cluster-configuration"`,
			Manifest: func() interface{} {
				return manifests.SecretWithProviderClusterConfig(
					providerClusterConfig, nil,
				)
			},
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("kube-system").Create(ctx, manifest.(*apiv1.Secret), metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				data, err := json.Marshal(manifest.(*apiv1.Secret))
				if err != nil {
					return err
				}
				_, err = kubeCl.CoreV1().Secrets("kube-system").Patch(ctx,
					"d8-provider-cluster-configuration",
					types.MergePatchType,
					data,
					metav1.PatchOptions{},
				)
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
