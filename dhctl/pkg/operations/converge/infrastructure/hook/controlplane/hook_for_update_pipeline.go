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

	"github.com/name212/govalue"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
	"github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/plan"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infrastructure/hook"
	infra_utils "github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infrastructure/utils"
)

type ClientSwitcher interface {
	SwitchClientsToAnotherNodeIfNeed(ctx context.Context, nodeName, ip string) error
}

type HookForUpdatePipeline struct {
	*Checker
	kubeGetter        kubernetes.KubeClientProviderWithCtx
	sshProvider       libcon.SSHProvider
	nodeToConverge    string
	oldMasterIPForSSH string
	commanderMode     bool
	immutableNode     bool
	clientSwitcher    ClientSwitcher
}

func NewHookForUpdatePipeline(
	kubeGetter kubernetes.KubeClientProviderWithCtx,
	sshProvider libcon.SSHProvider,
	nodeToHostForChecks map[string]string,
	sessionForNode SessionForNode,
	commanderMode bool,
	skipChecks bool,
	immutableNode bool,
) *HookForUpdatePipeline {
	checkers := []hook.NodeChecker{
		hook.NewKubeNodeReadinessChecker(kubeGetter),
	}

	// An immutable node answers no sshd: the check would fail on every master, and
	// what it proves — that the machine is alive and serving — the control plane
	// checker below proves through the cluster.
	if !commanderMode && !skipChecks && !immutableNode {
		checkers = append(
			checkers,
			NewSSHChecker(
				sshProvider,
				nodeToHostForChecks,
				sessionForNode,
			),
		)
	}

	checkers = append(checkers, NewManagerReadinessChecker(kubeGetter))
	checkers = append(checkers, NewStrongholdReadinessChecker(kubeGetter))

	checker := NewChecker(
		nodeToHostForChecks,
		checkers,
		"",
		DefaultConfirm,
	)

	return &HookForUpdatePipeline{
		Checker:       checker,
		kubeGetter:    kubeGetter,
		sshProvider:   sshProvider,
		commanderMode: commanderMode,
		immutableNode: immutableNode,
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

func (h *HookForUpdatePipeline) WithClientSwitcher(s ClientSwitcher) *HookForUpdatePipeline {
	h.clientSwitcher = s
	return h
}

func (h *HookForUpdatePipeline) BeforeAction(ctx context.Context, runner infrastructure.RunnerInterface) (bool, error) {
	if runner.GetChangesInPlan() != plan.HasDestructiveChanges {
		return false, nil
	}

	if !runner.HasVMDestruction() {
		dhlog.FromContext(ctx).InfoContext(ctx, "Plan has destructive changes, but not for a master instance VM. Skipping control plane hook actions.")
		return false, nil
	}

	if len(h.nodeToHostForChecks) == 0 {
		return false, ErrSingleMasterClusterInfrastructurePlanHasDestructiveChanges
	}

	err := h.IsAllNodesReady(ctx)
	if err != nil {
		return false, fmt.Errorf("not all nodes are ready: %v", err)
	}

	// use no strict because we can have situation when vm was destroyed
	// in previous run, but all resources not deleted. in this situation
	// we cannot have ssh ip and internal ip in state because infra util
	// delete output on remove vm
	// in restart operation we will get error with strict getting
	outputs, err := infrastructure.GetMasterNodeResultNoStrict(ctx, runner)
	if err != nil {
		return false, fmt.Errorf("failed to get master node pipeline outputs: %w", err)
	}

	// An immutable node is retired over the Kubernetes API alone, so a missing SSH
	// address says nothing about whether it can be done. Skipping on it would recreate
	// the VM while the old node is still a voting etcd member.
	masterIP := outputs.MasterIPForSSH
	if masterIP == "" && !h.immutableNode {
		h.oldMasterIPForSSH = ""
		dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("Got empty master IP for ssh for node %s.", h.nodeToConverge))
		return false, nil
	}

	if !govalue.IsNil(h.clientSwitcher) && h.nodeToConverge != "" {
		if err := h.clientSwitcher.SwitchClientsToAnotherNodeIfNeed(ctx, h.nodeToConverge, masterIP); err != nil {
			return false, err
		}
	}

	// The switch above moves an SSH-tunnelled client off the machine about to be
	// destroyed. An sshless converge has no session to move: whoever runs it decides
	// which apiserver the kubeconfig names, and if that is this machine the run loses
	// the cluster mid-flight. Saying so before the destruction beats a stack trace
	// after it — the converge state is saved, so a rerun continues where this stopped.
	if h.immutableNode {
		dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf(
			"About to destroy %s (%s). This converge talks to the cluster over the Kubernetes API: "+
				"if the kubeconfig points at this very node, the connection dies with it and the run stops. "+
				"Rerun converge afterwards — the saved state resumes from here.",
			h.nodeToConverge, masterIP))
	}

	h.oldMasterIPForSSH = masterIP

	kubeClient, err := h.kubeGetter.KubeClientCtx(ctx)
	if err != nil {
		return false, fmt.Errorf("Could not get kube client: %w", err)
	}

	err = removeControlPlaneRoleFromNode(ctx, kubeClient, h.kubeGetter, h.nodeToConverge, h.commanderMode, h.immutableNode)
	if err != nil {
		return false, fmt.Errorf("failed to remove control plane role from node '%s': %v", h.nodeToConverge, err)
	}

	err = infra_utils.DeleteNodeObjectFromCluster(ctx, kubeClient, h.nodeToConverge)
	if err != nil {
		return false, fmt.Errorf("failed to delete node object '%s' from cluster: %v\n", h.nodeToConverge, err)
	}

	return false, nil
}

// moveSessionToRecreatedNode puts the rebuilt master back in reach, under the converge user
// a rebuilt machine boots with. The state naming that node is written only after a
// successful apply, so the generation is not read from it but proven on the connection.
func (h *HookForUpdatePipeline) moveSessionToRecreatedNode(ctx context.Context, cl libcon.SSHClient, host session.Host) error {
	live := cl.Session()

	if h.oldMasterIPForSSH != "" {
		live.RemoveAvailableHosts(session.Host{Host: h.oldMasterIPForSSH, Name: h.nodeToConverge})
	}

	// The machine behind the name changed, and a keyed client built for the old one keeps
	// reporting itself alive on the legacy backend. Node deletion drops it the same way.
	if standalone, ok := h.sshProvider.(libcon.StandaloneClientProvider); ok {
		standalone.StopStandaloneClientFor(ctx, SSHCheckerClientKey(h.nodeToConverge))
	}

	if live.User == global.ConvergeUserName {
		live.AddAvailableHosts(host)
		return nil
	}

	if len(live.AvailableHosts()) > 0 {
		dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf(
			"Rebuilt node %s answers to %s while the clients run as %s. It joins when they move to its generation",
			h.nodeToConverge, global.ConvergeUserName, live.User))

		return nil
	}

	// No host of the current generation is left to talk to, so the clients follow the
	// rebuilt master instead of running out of hosts.
	if err := h.followRecreatedNode(ctx, cl, live, host); err != nil {
		return fmt.Errorf("move the clients to the recreated node %s: %w", h.nodeToConverge, err)
	}

	return nil
}

// followRecreatedNode moves the clients onto the rebuilt master as the converge user and
// proves that account answers, falling back to the user dhctl started with. A node whose
// provider dropped the account from its cloud-config still answers to that one.
func (h *HookForUpdatePipeline) followRecreatedNode(ctx context.Context, cl libcon.SSHClient, live *session.Session, host session.Host) error {
	// The converge user's sudo is NOPASSWD, so no password is sent to that account.
	err := switchAndCheck(ctx, h.sshProvider, sessionForHost(live, global.ConvergeUserName, "", host), cl.PrivateKeys())
	if err == nil {
		return nil
	}

	dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf(
		"Cannot connect to the rebuilt %s as %s: %v. Retrying as %s",
		h.nodeToConverge, global.ConvergeUserName, err, live.User))

	// The operator's account is the one the sudo password was given for.
	operator := sessionForHost(live, live.User, live.BecomePass, host)

	if retryErr := switchAndCheck(ctx, h.sshProvider, operator, cl.PrivateKeys()); retryErr != nil {
		return fmt.Errorf("connect as %s (%v), then as %s: %w", global.ConvergeUserName, err, live.User, retryErr)
	}

	return nil
}

func (h *HookForUpdatePipeline) AfterAction(ctx context.Context, runner infrastructure.RunnerInterface) error {
	if runner.GetChangesInPlan() != plan.HasDestructiveChanges {
		return nil
	}

	if !runner.HasVMDestruction() {
		dhlog.FromContext(ctx).InfoContext(ctx, "Plan has destructive changes, but not for a master instance VM. Skipping control plane hook actions.")
		return nil
	}

	outputs, err := infrastructure.GetMasterNodeResult(ctx, runner, nil)
	if err != nil {
		return fmt.Errorf("failed to get master node pipeline outputs: %w", err)
	}

	// Nothing to move for an immutable node: no session was ever pinned to it.
	if !h.commanderMode && !h.immutableNode {
		cl, err := h.sshProvider.Client(ctx)
		if err != nil {
			return fmt.Errorf("get ssh client to move the session to the recreated node: %w", err)
		}

		host := session.Host{Host: outputs.MasterIPForSSH, Name: h.nodeToConverge}
		if err := h.moveSessionToRecreatedNode(ctx, cl, host); err != nil {
			return err
		}
	}

	// Before waiting for the master node to be listed as a member of the etcd cluster,
	// we need to store the path to the Kubernetes data device to avoid deadlock.
	err = h.saveKubernetesDataDevicePath(ctx, outputs.KubeDataDevicePath)
	if err != nil {
		return fmt.Errorf("failed to save kubernetes data device path: %w", err)
	}

	kubeCl, err := h.kubeGetter.KubeClientCtx(ctx)
	if err != nil {
		return err
	}

	err = entity.WaitForSingleNodeBecomeReady(ctx, kubeCl, h.nodeToConverge)
	if err != nil {
		return fmt.Errorf("failed to wait for the master node '%s' to become Ready: %w", h.nodeToConverge, err)
	}

	err = waitEtcdHasMember(ctx, h.kubeGetter, h.nodeToConverge)
	if err != nil {
		return fmt.Errorf("failed to wait for the master node '%s' to be listed as etcd cluster member: %w", h.nodeToConverge, err)
	}

	err = retry.NewLoop("Check Stronghold readiness after node converge", 450, 1*time.Second).RunContext(ctx, func() error {
		ready, err := NewStrongholdReadinessChecker(h.kubeGetter).IsReady(ctx, h.nodeToConverge)
		if err != nil {
			return fmt.Errorf("failed to check Stronghold readiness: %w", err)
		}

		if !ready {
			return hook.ErrNotReady
		}

		return nil
	})
	if err != nil {
		return err
	}

	loopParams := retry.NewEmptyParams(
		retry.WithName("Check control-plane is ready on node '%s'", h.nodeToConverge),
		retry.WithAttempts(450),
		retry.WithWait(1*time.Second),
		retry.WithWhitelist(hook.ErrNotReady, ErrControlPlaneReadinessCheckTransient),
	)

	return retry.NewLoopWithParams(loopParams).RunContext(ctx, func() error {
		ready, err := NewManagerReadinessChecker(h.kubeGetter).IsReady(ctx, h.nodeToConverge)
		if err != nil {
			return fmt.Errorf("failed to check the master node '%s' readiness: %w", h.nodeToConverge, err)
		}

		if !ready {
			return hook.ErrNotReady
		}

		return nil
	})
}

func (h *HookForUpdatePipeline) IsReady() error {
	return nil
}

func (h *HookForUpdatePipeline) saveKubernetesDataDevicePath(ctx context.Context, devicePath string) error {
	getDevicePathManifest := func() any {
		return manifests.SecretMasterDevicePath(h.nodeToConverge, []byte(devicePath))
	}

	kubeClient, err := h.kubeGetter.KubeClientCtx(ctx)
	if err != nil {
		return fmt.Errorf("Could not get kube client: %w", err)
	}

	task := actions.ManifestTask{
		Name:     `Secret "d8-masters-kubernetes-data-device-path"`,
		Manifest: getDevicePathManifest,
		CreateFunc: func(ctx context.Context, manifest any) error {
			_, err := kubeClient.CoreV1().Secrets("d8-system").Create(ctx, manifest.(*apiv1.Secret), metav1.CreateOptions{})
			if err != nil {
				return err
			}

			return nil
		},
		UpdateFunc: func(ctx context.Context, manifest any) error {
			data, err := json.Marshal(manifest.(*apiv1.Secret))
			if err != nil {
				return err
			}

			_, err = kubeClient.CoreV1().Secrets("d8-system").Patch(
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

	loopParams := retry.NewEmptyParams(
		retry.WithName("Save Kubernetes data device path for node '%s'", h.nodeToConverge),
		retry.WithAttempts(450),
		retry.WithWait(1*time.Second),
		retry.WithWhitelist(actions.ErrManifestTaskTransient),
	)

	return retry.NewLoopWithParams(loopParams).
		RunContext(ctx, func() error {
			return task.CreateOrUpdate(ctx)
		})
}
