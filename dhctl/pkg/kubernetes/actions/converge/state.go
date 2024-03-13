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

package converge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

var ErrNoTerraformState = errors.New("Terraform state is not found in outputs.")

// Create secret for node with group settings only.
func CreateNodeTerraformState(kubeCl *client.KubernetesClient, nodeName, nodeGroup string, settings []byte) error {
	task := actions.ManifestTask{
		Name: fmt.Sprintf(`Secret "d8-node-terraform-state-%s"`, nodeName),
		Manifest: func() interface{} {
			return manifests.SecretWithNodeTerraformState(nodeName, nodeGroup, nil, settings)
		},
		CreateFunc: func(manifest interface{}) error {
			_, err := kubeCl.CoreV1().Secrets("d8-system").Create(context.TODO(), manifest.(*apiv1.Secret), metav1.CreateOptions{})
			return err
		},
		UpdateFunc: func(manifest interface{}) error {
			_, err := kubeCl.CoreV1().Secrets("d8-system").Update(context.TODO(), manifest.(*apiv1.Secret), metav1.UpdateOptions{})
			return err
		},
	}
	return retry.NewLoop(fmt.Sprintf("Create Terraform state for Node %q", nodeName), 45, 10*time.Second).Run(task.CreateOrUpdate)
}

func SaveNodeTerraformState(kubeCl *client.KubernetesClient, nodeName, nodeGroup string, tfState, settings []byte) error {
	if len(tfState) == 0 {
		return ErrNoTerraformState
	}

	task := actions.ManifestTask{
		Name: fmt.Sprintf(`Secret "d8-node-terraform-state-%s"`, nodeName),
		Manifest: func() interface{} {
			return manifests.SecretWithNodeTerraformState(nodeName, nodeGroup, tfState, settings)
		},
		CreateFunc: func(manifest interface{}) error {
			_, err := kubeCl.CoreV1().Secrets("d8-system").Create(context.TODO(), manifest.(*apiv1.Secret), metav1.CreateOptions{})
			return err
		},
		UpdateFunc: func(manifest interface{}) error {
			_, err := kubeCl.CoreV1().Secrets("d8-system").Update(context.TODO(), manifest.(*apiv1.Secret), metav1.UpdateOptions{})
			return err
		},
	}
	return retry.NewLoop(fmt.Sprintf("Save Terraform state for Node %q", nodeName), 45, 10*time.Second).Run(task.CreateOrUpdate)
}

func SaveMasterNodeTerraformState(kubeCl *client.KubernetesClient, nodeName string, tfState, devicePath []byte) error {
	if len(tfState) == 0 {
		return ErrNoTerraformState
	}

	getTerraformStateManifest := func() interface{} {
		return manifests.SecretWithNodeTerraformState(nodeName, MasterNodeGroupName, tfState, nil)
	}
	getDevicePathManifest := func() interface{} {
		return manifests.SecretMasterDevicePath(nodeName, devicePath)
	}

	tasks := []actions.ManifestTask{
		{
			Name:     fmt.Sprintf(`Secret "d8-node-terraform-state-%s"`, nodeName),
			Manifest: getTerraformStateManifest,
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("d8-system").Create(context.TODO(), manifest.(*apiv1.Secret), metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("d8-system").Update(context.TODO(), manifest.(*apiv1.Secret), metav1.UpdateOptions{})
				return err
			},
		},
		{
			Name:     `Secret "d8-masters-kubernetes-data-device-path"`,
			Manifest: getDevicePathManifest,
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("d8-system").Create(context.TODO(), manifest.(*apiv1.Secret), metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				data, err := json.Marshal(manifest.(*apiv1.Secret))
				if err != nil {
					return err
				}
				_, err = kubeCl.CoreV1().Secrets("d8-system").Patch(
					context.TODO(),
					"d8-masters-kubernetes-data-device-path",
					types.MergePatchType,
					data,
					metav1.PatchOptions{},
				)
				return err
			},
		},
	}

	return retry.NewLoop(fmt.Sprintf("Save Terraform state for master Node %s", nodeName), 45, 10*time.Second).Run(func() error {
		var allErrs *multierror.Error
		for _, task := range tasks {
			if err := task.CreateOrUpdate(); err != nil {
				allErrs = multierror.Append(allErrs, err)
			}
		}
		return allErrs.ErrorOrNil()
	})
}

func SaveClusterTerraformState(kubeCl *client.KubernetesClient, outputs *terraform.PipelineOutputs) error {
	if outputs == nil || len(outputs.TerraformState) == 0 {
		return ErrNoTerraformState
	}

	task := actions.ManifestTask{
		Name:     `Secret "d8-cluster-terraform-state"`,
		Manifest: func() interface{} { return manifests.SecretWithTerraformState(outputs.TerraformState) },
		CreateFunc: func(manifest interface{}) error {
			_, err := kubeCl.CoreV1().Secrets("d8-system").Create(context.TODO(), manifest.(*apiv1.Secret), metav1.CreateOptions{})
			return err
		},
		UpdateFunc: func(manifest interface{}) error {
			_, err := kubeCl.CoreV1().Secrets("d8-system").Update(context.TODO(), manifest.(*apiv1.Secret), metav1.UpdateOptions{})
			return err
		},
	}

	err := retry.NewLoop("Save Cluster Terraform state", 45, 10*time.Second).Run(task.CreateOrUpdate)
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

	return retry.NewLoop("Update cloud discovery data", 45, 10*time.Second).Run(func() error {
		_, err = kubeCl.CoreV1().Secrets("kube-system").Patch(
			context.TODO(),
			"d8-provider-cluster-configuration",
			types.MergePatchType,
			patch,
			metav1.PatchOptions{},
		)
		return err
	})
}

func DeleteTerraformState(kubeCl *client.KubernetesClient, secretName string) error {
	return retry.NewLoop(fmt.Sprintf("Delete Terraform state %s", secretName), 45, 10*time.Second).Run(func() error {
		return kubeCl.CoreV1().Secrets("d8-system").Delete(context.TODO(), secretName, metav1.DeleteOptions{})
	})
}

// NewClusterStateSaver returns StateSaver that saves intermediate terraform state to Secret.
// ErrNoIntermediateTerraformState is ignored because state file may become zero-sized during
// terraform apply.
//
// got FS event "/tmp/dhctl/static-node-dhctl.043483477.tfstate": WRITE
// '/tmp/dhctl/static-node-dhctl.043483477.tfstate' stat: 6492 bytes, mode: -rw-------
// openstack_networking_port_v2.port[0]: Creation complete after 7s [id=8e0aa9d1-07a4-4cfc-969b-96a52a8b182e]
// openstack_compute_instance_v2.node: Creating...
// got FS event "/tmp/dhctl/static-node-dhctl.043483477.tfstate": WRITE
// '/tmp/dhctl/static-node-dhctl.043483477.tfstate' stat: 6492 bytes, mode: -rw-------
// openstack_compute_instance_v2.node: Still creating... [10s elapsed]
// got FS event "/tmp/dhctl/static-node-dhctl.043483477.tfstate": WRITE
// '/tmp/dhctl/static-node-dhctl.043483477.tfstate' stat: 0 bytes, mode: -rw-------
// got FS event "/tmp/dhctl/static-node-dhctl.043483477.tfstate": WRITE
// '/tmp/dhctl/static-node-dhctl.043483477.tfstate' stat: 8840 bytes, mode: -rw-------

var (
	_ terraform.SaverDestination = &ClusterStateSaver{}
	_ terraform.SaverDestination = &NodeStateSaver{}
)

type ClusterStateSaver struct {
	kubeCl *client.KubernetesClient
}

func NewClusterStateSaver(kubeCl *client.KubernetesClient) *ClusterStateSaver {
	return &ClusterStateSaver{
		kubeCl: kubeCl,
	}
}

func (s *ClusterStateSaver) SaveState(outputs *terraform.PipelineOutputs) error {
	if outputs == nil || len(outputs.TerraformState) == 0 {
		return nil
	}

	task := actions.ManifestTask{
		Name: `Secret "d8-cluster-terraform-state"`,
		PatchData: func() interface{} {
			return manifests.PatchWithTerraformState(outputs.TerraformState)
		},
		PatchFunc: func(patch []byte) error {
			// MergePatch is used because we need to replace one field in "data".
			_, err := s.kubeCl.CoreV1().Secrets("d8-system").Patch(
				context.TODO(),
				manifests.TerraformClusterStateName,
				types.MergePatchType,
				patch,
				metav1.PatchOptions{},
			)
			return err
		},
	}

	log.DebugF("Intermediate save base infra in cluster...\n")
	err := retry.NewSilentLoop("Save Cluster intermediate Terraform state", 45, 10*time.Second).Run(task.Patch)
	msg := "Intermediate base infra was saved in cluster\n"
	if err != nil {
		msg = fmt.Sprintf("Intermediate base infra was not saved in cluster: %v\n", err)
	}

	log.DebugF(msg)
	return err
}

type NodeStateSaver struct {
	kubeCl            *client.KubernetesClient
	nodeName          string
	nodeGroup         string
	nodeGroupSettings []byte
}

func NewNodeStateSaver(kubeCl *client.KubernetesClient, nodeName, nodeGroup string, nodeGroupSettings []byte) *NodeStateSaver {
	return &NodeStateSaver{
		kubeCl:            kubeCl,
		nodeName:          nodeName,
		nodeGroup:         nodeGroup,
		nodeGroupSettings: nodeGroupSettings,
	}
}

// SaveState is a method to patch Secret with node state.
// It patches a "node-tf-state" key with terraform state or create a new secret if new node is created.
//
// settings can be nil for master node.
//
// The difference between master node and static node: master node has
// no key "node-group-settings.json" with group settings.
func (s *NodeStateSaver) SaveState(outputs *terraform.PipelineOutputs) error {
	if outputs == nil || len(outputs.TerraformState) == 0 {
		return nil
	}

	task := actions.ManifestTask{
		Name: fmt.Sprintf(`Secret "d8-node-terraform-state-%s"`, s.nodeName),
		Manifest: func() interface{} {
			return manifests.SecretWithNodeTerraformState(s.nodeName, s.nodeGroup, outputs.TerraformState, s.nodeGroupSettings)
		},
		CreateFunc: func(manifest interface{}) error {
			_, err := s.kubeCl.CoreV1().Secrets("d8-system").Create(context.TODO(), manifest.(*apiv1.Secret), metav1.CreateOptions{})
			return err
		},
		PatchData: func() interface{} {
			return manifests.PatchWithNodeTerraformState(outputs.TerraformState)
		},
		PatchFunc: func(patchData []byte) error {
			secretName := manifests.SecretNameForNodeTerraformState(s.nodeName)
			// MergePatch is used because we need to replace one field in "data".
			_, err := s.kubeCl.CoreV1().Secrets("d8-system").Patch(context.TODO(), secretName, types.MergePatchType, patchData, metav1.PatchOptions{})
			return err
		},
	}
	taskName := fmt.Sprintf("Save intermediate Terraform state for Node %q", s.nodeName)
	log.DebugF("Intermediate save state for node %s in cluster...\n", s.nodeName)
	err := retry.NewSilentLoop(taskName, 45, 10*time.Second).Run(task.PatchOrCreate)
	msg := fmt.Sprintf("Intermediate state for node %s was saved in cluster\n", s.nodeName)
	if err != nil {
		msg = fmt.Sprintf("Intermediate state for node %s was not saved in cluster: %v\n", s.nodeName, err)
	}

	log.DebugF(msg)

	return err
}
