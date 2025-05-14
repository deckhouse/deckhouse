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

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	infra_utils "github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infrastructure/utils"
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
	return false, nil
}

func (h *HookForDestroyPipeline) IsReady() error {
	return nil
}

func (h *HookForDestroyPipeline) AfterAction(ctx context.Context, runner infrastructure.RunnerInterface) error {
	return nil
}
