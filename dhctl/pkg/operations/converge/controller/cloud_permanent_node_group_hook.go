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

package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	infra_utils "github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infrastructure/utils"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CloudPermanentNodeGroupHook struct {
	getter        kubernetes.KubeClientProvider
	nodeToDestroy string
}

type HookForDestroyPipeline struct {
	getter            kubernetes.KubeClientProvider
	nodeToDestroy     string
	oldMasterIPForSSH string
	commanderMode     bool
}

func NewHookForDestroyPipeline(getter kubernetes.KubeClientProvider, nodeToDestroy string, commanderMode bool) *HookForDestroyPipeline {
	return &HookForDestroyPipeline{
		getter:        getter,
		nodeToDestroy: nodeToDestroy,
		commanderMode: commanderMode,
	}
}

func (h *HookForDestroyPipeline) BeforeAction(ctx context.Context, runner infrastructure.RunnerInterface) (runPostAction bool, err error) {
	err = infra_utils.TryToDrainNode(ctx, h.getter.KubeClient(), h.nodeToDestroy, infra_utils.GetDrainConfirmation(h.commanderMode), infra_utils.DrainOptions{Force: false})
	if err != nil {
		return false, err
	}
	err = deleteNode(ctx, h.getter.KubeClient(), h.nodeToDestroy)
	if err != nil {
		return false, fmt.Errorf("failed to delete object node '%s' from cluster: %v\n", h.nodeToDestroy, err)
	}
	return false, nil
}

func (h *HookForDestroyPipeline) IsReady() error {
	return nil
}

func (h *HookForDestroyPipeline) AfterAction(ctx context.Context, runner infrastructure.RunnerInterface) error {
	return nil
}

func deleteNode(ctx context.Context, kubeCl *client.KubernetesClient, nodeName string) error {
	return retry.NewLoop(
		fmt.Sprintf("Delete node %s", nodeName),
		10,
		5*time.Second,
	).RunContext(ctx, func() error {
		err := kubeCl.CoreV1().Nodes().Delete(ctx, nodeName, metav1.DeleteOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				log.InfoF("Node '%s' already deleted. Skip\n", nodeName)
				return nil
			}
			return err
		}

		log.InfoF("Node '%s' successfully deleted from cluster\n", nodeName)
		return nil
	})
}
