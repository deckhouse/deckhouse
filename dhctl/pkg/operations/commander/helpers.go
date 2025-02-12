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

package commander

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

func NewErrClusterManagedByAnotherCommander(managedByCommanderUUID, requiredCommanderUUID uuid.UUID) error {
	return fmt.Errorf("cluster managed by another commander %s unable to perform operations from your commander %s", managedByCommanderUUID.String(), requiredCommanderUUID.String())
}

func doCheckShouldUpdateCommanderUUID(cm *v1.ConfigMap, requiredCommanderUUID uuid.UUID) (bool, error) {
	if val, hasKey := cm.Data[manifests.CommanderUUIDCmKey]; hasKey {
		valUUID, err := uuid.Parse(val)
		if err != nil {
			// ignore incorrect value, and take over with required commander uuid
			return true, nil
		}
		if valUUID != requiredCommanderUUID {
			return false, NewErrClusterManagedByAnotherCommander(valUUID, requiredCommanderUUID)
		}
		return false, nil
	}
	// if no commander uuid data found then should update
	return true, nil
}

func CheckShouldUpdateCommanderUUID(ctx context.Context, kubeCl *client.KubernetesClient, requiredCommanderUUID uuid.UUID) (bool, error) {
	// FIXME(dhctl-for-commander): commander uuid currently optional, make it required later
	if requiredCommanderUUID == uuid.Nil {
		return false, nil
	}
	cm, err := kubeCl.CoreV1().ConfigMaps(manifests.CommanderUUIDCmNamespace).Get(ctx, manifests.CommanderUUIDCm, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// if no commander uuid found then should update
			return true, nil
		}
		return false, fmt.Errorf("unable to get cm/%s in ns/%s: %w", manifests.CommanderUUIDCm, manifests.CommanderUUIDCmNamespace, err)
	}
	return doCheckShouldUpdateCommanderUUID(cm, requiredCommanderUUID)
}

func ConstructManagedByCommanderConfigMapTask(commanderUUID uuid.UUID, kubeCl *client.KubernetesClient) actions.ManifestTask {
	return actions.ManifestTask{
		Name: `ConfigMap "d8-commander-uuid"`,
		Manifest: func() interface{} {
			return manifests.CommanderUUIDConfigMap(commanderUUID.String())
		},
		CreateFunc: func(manifest interface{}) error {
			_, err := kubeCl.CoreV1().ConfigMaps(manifests.CommanderUUIDCmNamespace).Create(context.TODO(), manifest.(*v1.ConfigMap), metav1.CreateOptions{})
			return err
		},
		UpdateFunc: func(manifest interface{}) error {
			existingCm, err := kubeCl.CoreV1().ConfigMaps(manifests.CommanderUUIDCmNamespace).Get(context.TODO(), manifests.CommanderUUIDCm, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("unable to get existing cm: %w", err)
			}

			shouldUpdate, err := doCheckShouldUpdateCommanderUUID(existingCm, commanderUUID)
			if err != nil {
				return fmt.Errorf("managed by commander check failed: %w", err)
			}

			if shouldUpdate {
				_, err = kubeCl.CoreV1().ConfigMaps(manifests.CommanderUUIDCmNamespace).Update(context.TODO(), manifest.(*v1.ConfigMap), metav1.UpdateOptions{})
				return err
			}
			return nil
		},
	}
}

func DeleteManagedByCommanderConfigMap(ctx context.Context, kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Delete commander-uuid ConfigMap", 20, 5*time.Second).WithShowError(false).Run(func() error {
		err := kubeCl.CoreV1().ConfigMaps(manifests.CommanderUUIDCmNamespace).Delete(ctx, manifests.CommanderUUIDCm, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		return nil
	})
}
