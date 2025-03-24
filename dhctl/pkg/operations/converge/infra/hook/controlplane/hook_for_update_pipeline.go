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

package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	flantkubeclient "github.com/flant/kube-client/client"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infra/hook"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

type HookForUpdatePipeline struct {
	*Checker
	kubeGetter        kubernetes.KubeClientProvider
	nodeToConverge    string
	oldMasterIPForSSH string
	commanderMode     bool
}

func NewHookForUpdatePipeline(
	kubeGetter kubernetes.KubeClientProvider,
	nodeToHostForChecks map[string]string,
	clusterUUID string,
	commanderMode bool,
) *HookForUpdatePipeline {
	checkers := []hook.NodeChecker{
		hook.NewKubeNodeReadinessChecker(kubeGetter),
	}

	if !commanderMode {
		cl := kubeGetter.KubeClient().NodeInterfaceAsSSHClient()
		if cl == nil {
			panic("Node interface is not ssh")
		}

		checkers = append(
			checkers,
			NewKubeProxyChecker().
				WithExternalIPs(nodeToHostForChecks).
				WithClusterUUID(clusterUUID).
				WithSSHCredentials(session.Input{
					User:        cl.Settings.User,
					Port:        cl.Settings.Port,
					BastionHost: cl.Settings.BastionHost,
					BastionPort: cl.Settings.BastionPort,
					BastionUser: cl.Settings.BastionUser,
					ExtraArgs:   cl.Settings.ExtraArgs,
					BecomePass:  cl.Settings.BecomePass,
				}, cl.PrivateKeys...))
	}

	checkers = append(checkers, NewManagerReadinessChecker(kubeGetter))
	checker := NewChecker(nodeToHostForChecks, checkers, "", DefaultConfirm)

	return &HookForUpdatePipeline{
		Checker:       checker,
		kubeGetter:    kubeGetter,
		commanderMode: commanderMode,
	}
}

func (h *HookForUpdatePipeline) WithSourceCommandName(name string) *HookForUpdatePipeline {
	h.sourceCommandName = name
	return h
}

func (h *HookForUpdatePipeline) WithNodeToConverge(nodeToConverge string) *HookForUpdatePipeline {
	h.nodeToConverge = nodeToConverge
	return h
}

func (h *HookForUpdatePipeline) WithConfirm(confirm func(msg string) bool) *HookForUpdatePipeline {
	h.confirm = confirm
	return h
}

func (h *HookForUpdatePipeline) BeforeAction(ctx context.Context, runner terraform.RunnerInterface) (bool, error) {
	if runner.GetChangesInPlan() != terraform.PlanHasDestructiveChanges {
		return false, nil
	}

	if len(h.nodeToHostForChecks) == 0 {
		return false, ErrSingleMasterClusterTerraformPlanHasDestructiveChanges
	}

	err := h.IsAllNodesReady(ctx)
	if err != nil {
		return false, fmt.Errorf("not all nodes are ready: %v", err)
	}

	err = removeControlPlaneRoleFromNode(ctx, h.kubeGetter.KubeClient(), h.nodeToConverge)
	if err != nil {
		return false, fmt.Errorf("failed to remove control plane role from node '%s': %v", h.nodeToConverge, err)
	}

	outputs, err := terraform.GetMasterNodeResult(ctx, runner)
	if err != nil {
		log.ErrorF("Get master node pipeline outputs: %v", err)
	}

	h.oldMasterIPForSSH = outputs.MasterIPForSSH

	return false, nil
}

func (h *HookForUpdatePipeline) AfterAction(ctx context.Context, runner terraform.RunnerInterface) error {
	if runner.GetChangesInPlan() != terraform.PlanHasDestructiveChanges {
		return nil
	}

	outputs, err := terraform.GetMasterNodeResult(ctx, runner)
	if err != nil {
		return fmt.Errorf("failed to get master node pipeline outputs: %v", err)
	}

	if !h.commanderMode {
		cl := h.kubeGetter.KubeClient().NodeInterfaceAsSSHClient()
		if cl == nil {
			panic("Node interface is not ssh")
		}

		cl.Settings.RemoveAvailableHosts(session.Host{Host: h.oldMasterIPForSSH, Name: h.nodeToConverge})
		cl.Settings.AddAvailableHosts(session.Host{Host: outputs.MasterIPForSSH, Name: h.nodeToConverge})

	}

	// Before waiting for the master node to be listed as a member of the etcd cluster,
	// we need to store the path to the Kubernetes data device to avoid deadlock.
	err = h.saveKubernetesDataDevicePath(ctx, outputs.KubeDataDevicePath)
	if err != nil {
		return fmt.Errorf("failed to save kubernetes data device path: %v", err)
	}

	err = waitEtcdHasMember(ctx, h.kubeGetter.KubeClient().KubeClient.(*flantkubeclient.Client), h.nodeToConverge)
	if err != nil {
		return fmt.Errorf("failed to wait for the master node '%s' to be listed as etcd cluster member: %v", h.nodeToConverge, err)
	}

	err = retry.NewLoop(fmt.Sprintf("Check the master node '%s' is ready", h.nodeToConverge), 45, 10*time.Second).RunContext(ctx, func() error {
		ready, err := NewManagerReadinessChecker(h.kubeGetter).IsReady(ctx, h.nodeToConverge)
		if err != nil {
			return fmt.Errorf("failed to check the master node '%s' readiness: %v", h.nodeToConverge, err)
		}

		if !ready {
			return hook.ErrNotReady
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (h *HookForUpdatePipeline) IsReady() error {
	return nil
}

func (h *HookForUpdatePipeline) saveKubernetesDataDevicePath(ctx context.Context, devicePath string) error {
	getDevicePathManifest := func() interface{} {
		return manifests.SecretMasterDevicePath(h.nodeToConverge, []byte(devicePath))
	}

	task := actions.ManifestTask{
		Name:     `Secret "d8-masters-kubernetes-data-device-path"`,
		Manifest: getDevicePathManifest,
		CreateFunc: func(manifest interface{}) error {
			_, err := h.kubeGetter.KubeClient().CoreV1().Secrets("d8-system").Create(ctx, manifest.(*apiv1.Secret), metav1.CreateOptions{})
			if err != nil {
				return err
			}

			return nil
		},
		UpdateFunc: func(manifest interface{}) error {
			data, err := json.Marshal(manifest.(*apiv1.Secret))
			if err != nil {
				return err
			}

			_, err = h.kubeGetter.KubeClient().CoreV1().Secrets("d8-system").Patch(
				ctx,
				"d8-masters-kubernetes-data-device-path",
				types.MergePatchType,
				data,
				metav1.PatchOptions{},
			)
			if err != nil {
				return err
			}

			return nil
		},
	}

	return retry.NewLoop(fmt.Sprintf("Save Kubernetes data device path for node '%s'", h.nodeToConverge), 45, 10*time.Second).
		RunContext(ctx, func() error {
			err := task.CreateOrUpdate()
			if err != nil {
				return err
			}

			return nil
		})
}
