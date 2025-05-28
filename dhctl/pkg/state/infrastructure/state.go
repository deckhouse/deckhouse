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

package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	v1 "k8s.io/api/core/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

var ErrNoInfrastructureState = errors.New("Infrastructure state is not found in outputs.")

func GetClusterStateFromCluster(ctx context.Context, kubeCl *client.KubernetesClient) ([]byte, error) {
	var st []byte
	err := retry.NewLoop("Get Cluster infrastructure state from Kubernetes cluster", 5, 5*time.Second).
		RunContext(ctx, func() error {
			clusterStateSecret, err := kubeCl.CoreV1().Secrets(global.D8SystemNamespace).Get(ctx, manifests.InfrastructureClusterStateName, metav1.GetOptions{})
			if err != nil {
				if k8errors.IsNotFound(err) {
					// Return empty state, if there is no state in cluster. Need to skip cluster state apply in converge.
					return nil
				}
				return err
			}

			st = clusterStateSecret.Data["cluster-tf-state.json"]
			return nil
		})
	return st, err
}

func GetClusterUUID(ctx context.Context, kubeCl *client.KubernetesClient) (string, error) {
	var clusterUUID string
	err := retry.NewLoop("Get Cluster UUID from the Kubernetes cluster", 5, 5*time.Second).
		RunContext(ctx, func() error {
			uuidConfigMap, err := kubeCl.CoreV1().ConfigMaps("kube-system").Get(ctx, "d8-cluster-uuid", metav1.GetOptions{})
			if err != nil {
				return err
			}

			clusterUUID = uuidConfigMap.Data["cluster-uuid"]
			return nil
		})
	return clusterUUID, err
}

func GetNodesStateSecretsFromCluster(ctx context.Context, kubeCl *client.KubernetesClient) ([]*v1.Secret, error) {
	var secrets []*v1.Secret
	err := retry.NewLoop("Get Nodes infrastructure state from Kubernetes cluster", 5, 5*time.Second).RunContext(ctx, func() error {
		nodeStateSecrets, err := kubeCl.CoreV1().Secrets(global.D8SystemNamespace).List(ctx, metav1.ListOptions{LabelSelector: manifests.NodeInfrastructureStateLabelKey})
		if err != nil {
			return err
		}

		for _, nodeState := range nodeStateSecrets.Items {
			name := nodeState.Labels["node.deckhouse.io/node-name"]
			if name == "" {
				return fmt.Errorf("can't determine Node name for %q secret", nodeState.Name)
			}

			if _, ok := nodeState.Labels[global.InfrastructureStateBackupLabelKey]; ok {
				log.DebugF("Found backup state secret %s for node: %s. Skip.\n", nodeState.Name, name)
				continue
			}

			nodeGroup := nodeState.Labels["node.deckhouse.io/node-group"]
			if nodeGroup == "" {
				return fmt.Errorf("can't determine NodeGroup for %q secret", nodeState.Name)
			}
			secrets = append(secrets, &nodeState)
		}
		return nil
	})
	return secrets, err
}

func GetNodesStateFromCluster(ctx context.Context, kubeCl *client.KubernetesClient) (map[string]state.NodeGroupInfrastructureState, error) {
	secrets, err := GetNodesStateSecretsFromCluster(ctx, kubeCl)
	if err != nil {
		return nil, err
	}

	extractedState := make(map[string]state.NodeGroupInfrastructureState, len(secrets))

	for _, nodeState := range secrets {
		name := nodeState.Labels["node.deckhouse.io/node-name"]
		if name == "" {
			return nil, fmt.Errorf("can't determine Node name for %q secret", nodeState.Name)
		}

		nodeGroup := nodeState.Labels["node.deckhouse.io/node-group"]
		if nodeGroup == "" {
			return nil, fmt.Errorf("can't determine NodeGroup for %q secret", nodeState.Name)
		}

		if _, ok := extractedState[nodeGroup]; !ok {
			extractedState[nodeGroup] = state.NodeGroupInfrastructureState{State: make(map[string][]byte)}
		}

		// TODO: validate, that all secrets from node group have same node-group-settings.json
		nodeGroupInfrastructureState := extractedState[nodeGroup]
		nodeGroupInfrastructureState.Settings = nodeState.Data["node-group-settings.json"]

		st := nodeState.Data["node-tf-state.json"]
		nodeGroupInfrastructureState.State[name] = st

		extractedState[nodeGroup] = nodeGroupInfrastructureState
	}

	return extractedState, err
}

func SaveNodeInfrastructureState(ctx context.Context, kubeCl *client.KubernetesClient, nodeName, nodeGroup string, tfState, settings []byte, logger log.Logger) error {
	if len(tfState) == 0 {
		return ErrNoInfrastructureState
	}

	task := actions.ManifestTask{
		Name: fmt.Sprintf(`Secret "d8-node-terraform-state-%s"`, nodeName),
		Manifest: func() interface{} {
			return manifests.SecretWithNodeInfrastructureState(nodeName, nodeGroup, tfState, settings)
		},
		CreateFunc: func(manifest interface{}) error {
			_, err := kubeCl.CoreV1().Secrets("d8-system").Create(ctx, manifest.(*v1.Secret), metav1.CreateOptions{})
			return err
		},
		UpdateFunc: func(manifest interface{}) error {
			_, err := kubeCl.CoreV1().Secrets("d8-system").Update(ctx, manifest.(*v1.Secret), metav1.UpdateOptions{})
			return err
		},
	}
	return retry.NewLoop(fmt.Sprintf("Save infrastructure state for Node %q", nodeName), 45, 10*time.Second).
		WithLogger(logger).RunContext(ctx, task.CreateOrUpdate)
}

func SaveMasterNodeInfrastructureState(ctx context.Context, kubeCl *client.KubernetesClient, nodeName string, tfState, devicePath []byte) error {
	if len(tfState) == 0 {
		return ErrNoInfrastructureState
	}

	getInfrastructureStateManifest := func() interface{} {
		return manifests.SecretWithNodeInfrastructureState(nodeName, global.MasterNodeGroupName, tfState, nil)
	}
	getDevicePathManifest := func() interface{} {
		return manifests.SecretMasterDevicePath(nodeName, devicePath)
	}

	tasks := []actions.ManifestTask{
		{
			Name:     fmt.Sprintf(`Secret "d8-node-terraform-state-%s"`, nodeName),
			Manifest: getInfrastructureStateManifest,
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("d8-system").Create(ctx, manifest.(*v1.Secret), metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("d8-system").Update(ctx, manifest.(*v1.Secret), metav1.UpdateOptions{})
				return err
			},
		},
		{
			Name:     `Secret "d8-masters-kubernetes-data-device-path"`,
			Manifest: getDevicePathManifest,
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("d8-system").Create(ctx, manifest.(*v1.Secret), metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				data, err := json.Marshal(manifest.(*v1.Secret))
				if err != nil {
					return err
				}
				_, err = kubeCl.CoreV1().Secrets("d8-system").Patch(
					ctx,
					"d8-masters-kubernetes-data-device-path",
					types.MergePatchType,
					data,
					metav1.PatchOptions{},
				)
				return err
			},
		},
	}

	return retry.NewLoop(fmt.Sprintf("Save infrastructure state for master Node %s", nodeName), 45, 10*time.Second).RunContext(ctx, func() error {
		var allErrs *multierror.Error
		for _, task := range tasks {
			if err := task.CreateOrUpdate(); err != nil {
				allErrs = multierror.Append(allErrs, err)
			}
		}
		return allErrs.ErrorOrNil()
	})
}

func SaveClusterInfrastructureState(ctx context.Context, kubeCl *client.KubernetesClient, outputs *infrastructure.PipelineOutputs) error {
	if outputs == nil || len(outputs.InfrastructureState) == 0 {
		return ErrNoInfrastructureState
	}

	task := actions.ManifestTask{
		Name:     `Secret "d8-cluster-terraform-state"`,
		Manifest: func() interface{} { return manifests.SecretWithInfrastructureState(outputs.InfrastructureState) },
		CreateFunc: func(manifest interface{}) error {
			_, err := kubeCl.CoreV1().Secrets("d8-system").Create(ctx, manifest.(*v1.Secret), metav1.CreateOptions{})
			return err
		},
		UpdateFunc: func(manifest interface{}) error {
			_, err := kubeCl.CoreV1().Secrets("d8-system").Update(ctx, manifest.(*v1.Secret), metav1.UpdateOptions{})
			return err
		},
	}

	err := retry.NewLoop("Save Cluster infrastructure state", 45, 10*time.Second).RunContext(ctx, task.CreateOrUpdate)
	if err != nil {
		return err
	}

	patch, err := json.Marshal(map[string]interface{}{
		"data": map[string]interface{}{
			"cloud-provider-discovery-data.json": outputs.CloudDiscovery,
		},
	})
	if err != nil {
		return err
	}

	return retry.NewLoop("Update cloud discovery data", 45, 10*time.Second).RunContext(ctx, func() error {
		_, err = kubeCl.CoreV1().Secrets("kube-system").Patch(
			ctx,
			"d8-provider-cluster-configuration",
			types.MergePatchType,
			patch,
			metav1.PatchOptions{},
		)
		return err
	})
}

func DeleteInfrastructureState(ctx context.Context, kubeCl *client.KubernetesClient, secretName string) error {
	return retry.NewLoop(fmt.Sprintf("Delete infrastructure state %s", secretName), 45, 10*time.Second).
		RunContext(ctx, func() error {
			return kubeCl.CoreV1().Secrets("d8-system").Delete(ctx, secretName, metav1.DeleteOptions{})
		})
}
