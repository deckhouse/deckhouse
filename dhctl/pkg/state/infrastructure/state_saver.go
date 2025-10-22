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
	"fmt"
	"time"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

// NewClusterStateSaver returns StateSaver that saves intermediate infrastructure state to Secret.
// ErrNoIntermediateTerraformState is ignored because state file may become zero-sized during
// infrastructure apply.
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
	_ infrastructure.SaverDestination = &ClusterStateSaver{}
	_ infrastructure.SaverDestination = &NodeStateSaver{}
)

type ClusterStateSaver struct {
	getter kubernetes.KubeClientProvider
}

func NewClusterStateSaver(getter kubernetes.KubeClientProvider) *ClusterStateSaver {
	return &ClusterStateSaver{
		getter: getter,
	}
}

func (s *ClusterStateSaver) SaveState(outputs *infrastructure.PipelineOutputs) error {
	if outputs == nil || len(outputs.InfrastructureState) == 0 {
		return nil
	}

	task := actions.ManifestTask{
		Name: `Secret "d8-cluster-terraform-state"`,
		PatchData: func() interface{} {
			return manifests.PatchWithInfrastructureState(outputs.InfrastructureState)
		},
		PatchFunc: func(patch []byte) error {
			// MergePatch is used because we need to replace one field in "data".
			_, err := s.getter.KubeClient().CoreV1().Secrets("d8-system").Patch(
				context.TODO(),
				manifests.InfrastructureClusterStateName,
				types.MergePatchType,
				patch,
				metav1.PatchOptions{},
			)
			return err
		},
	}

	log.DebugF("Intermediate save base infra in cluster...\n")
	err := retry.NewSilentLoop("Save Cluster intermediate infrastructure state", 45, 10*time.Second).Run(task.Patch)
	msg := "Intermediate base infra was saved in cluster\n"
	if err != nil {
		msg = fmt.Sprintf("Intermediate base infra was not saved in cluster: %v\n", err)
	}

	log.DebugF(msg)
	return err
}

type NodeStateSaver struct {
	getter            kubernetes.KubeClientProvider
	nodeName          string
	nodeGroup         string
	nodeGroupSettings []byte
}

func NewNodeStateSaver(getter kubernetes.KubeClientProvider, nodeName, nodeGroup string, nodeGroupSettings []byte) *NodeStateSaver {
	return &NodeStateSaver{
		getter:            getter,
		nodeName:          nodeName,
		nodeGroup:         nodeGroup,
		nodeGroupSettings: nodeGroupSettings,
	}
}

// SaveState is a method to patch Secret with node state.
// It patches a "node-tf-state" key with infrastructure state or create a new secret if new node is created.
//
// settings can be nil for master node.
//
// The difference between master node and static node: master node has
// no key "node-group-settings.json" with group settings.
func (s *NodeStateSaver) SaveState(outputs *infrastructure.PipelineOutputs) error {
	if outputs == nil || len(outputs.InfrastructureState) == 0 {
		return nil
	}

	task := actions.ManifestTask{
		Name: fmt.Sprintf(`Secret "d8-node-terraform-state-%s"`, s.nodeName),
		Manifest: func() interface{} {
			return manifests.SecretWithNodeInfrastructureState(s.nodeName, s.nodeGroup, outputs.InfrastructureState, s.nodeGroupSettings)
		},
		CreateFunc: func(manifest interface{}) error {
			_, err := s.getter.KubeClient().CoreV1().Secrets("d8-system").Create(context.TODO(), manifest.(*apiv1.Secret), metav1.CreateOptions{})
			return err
		},
		PatchData: func() interface{} {
			return manifests.PatchWithInfrastructureState(outputs.InfrastructureState)
		},
		PatchFunc: func(patchData []byte) error {
			secretName := manifests.SecretNameForNodeInfrastructureState(s.nodeName)
			// MergePatch is used because we need to replace one field in "data".
			_, err := s.getter.KubeClient().CoreV1().Secrets("d8-system").Patch(context.TODO(), secretName, types.MergePatchType, patchData, metav1.PatchOptions{})
			return err
		},
	}
	taskName := fmt.Sprintf("Save intermediate infrastructure state for Node %q", s.nodeName)
	log.DebugF("Intermediate save state for node %s in cluster...\n", s.nodeName)
	err := retry.NewSilentLoop(taskName, 45, 10*time.Second).Run(task.PatchOrCreate)
	msg := fmt.Sprintf("Intermediate state for node %s was saved in cluster\n", s.nodeName)
	if err != nil {
		msg = fmt.Sprintf("Intermediate state for node %s was not saved in cluster: %v\n", s.nodeName, err)
	}

	log.DebugF(msg)

	return err
}
