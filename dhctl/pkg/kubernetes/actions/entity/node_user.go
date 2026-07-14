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

package entity

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/name212/govalue"
	corev1 "k8s.io/api/core/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/selection"

	"github.com/deckhouse/lib-dhctl/pkg/retry"
	sdk "github.com/deckhouse/module-sdk/pkg/utils"

	v1 "github.com/deckhouse/deckhouse/dhctl/pkg/apis/deckhouse/v1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

var createUpdateNodeUsersDefaultOpts = retry.AttemptsWithWaitOpts(450, 1*time.Second)

// errNodeUserSaveTransient marks a create/update failure that may succeed on retry (a
// resource-version conflict, a transient API error, a kube-client provisioning hiccup, or an
// admission-webhook rejection that depends on another resource's state, e.g. a NodeGroup the
// NodeUser references still being reconciled), as opposed to a permanent authorization
// failure that fails identically on every attempt.
var errNodeUserSaveTransient = fmt.Errorf("save NodeUser: transient error, may succeed on retry")

// wrapNodeUserSaveErr tags err as transient unless it is a permanent authorization failure, so
// the retry loop can whitelist errNodeUserSaveTransient.
func wrapNodeUserSaveErr(prefix string, err error) error {
	if k8errors.IsForbidden(err) || k8errors.IsUnauthorized(err) {
		return fmt.Errorf("%s: %w", prefix, err)
	}
	return fmt.Errorf("%w: %s: %w", errNodeUserSaveTransient, prefix, err)
}

func CreateOrUpdateNodeUser(ctx context.Context, kubeProvider kubernetes.KubeClientProviderWithCtx, nodeUser *v1.NodeUser, loopParams retry.Params) error {
	nodeUserResource, err := sdk.ToUnstructured(nodeUser)
	if err != nil {
		return fmt.Errorf("Failed to convert NodeUser to unstructured: %w", err)
	}

	loopParams = retry.SafeCloneOrNewParams(loopParams, createUpdateNodeUsersDefaultOpts...).
		Clone(
			retry.WithName("Save NodeUser '%s'", nodeUser.GetName()),
			retry.WithWhitelist(errNodeUserSaveTransient),
		)

	return retry.NewLoopWithParams(loopParams).RunContext(ctx, func() error {
		kubeCl, err := kubeProvider.KubeClientCtx(ctx)
		if err != nil {
			return fmt.Errorf("%w: %w", errNodeUserSaveTransient, err)
		}

		if err := createNodeUser(ctx, kubeCl, nodeUserResource); err != nil {
			if k8errors.IsAlreadyExists(err) {
				if err := updateNodeUser(ctx, kubeCl, nodeUserResource); err != nil {
					return wrapNodeUserSaveErr("Failed to update NodeUser", err)
				}

				return nil
			}

			return wrapNodeUserSaveErr("Failed to create NodeUser", err)
		}

		return nil
	})
}

func DeleteNodeUser(ctx context.Context, kubeProvider kubernetes.KubeClientProviderWithCtx, name string) error {
	processName := fmt.Sprintf("Delete NodeUser %s", name)
	return retry.NewLoop(processName, 450, 1*time.Second).RunContext(ctx, func() error {
		kubeCl, err := kubeProvider.KubeClientCtx(ctx)
		if err != nil {
			return err
		}
		err = kubeCl.Dynamic().Resource(v1.NodeUserGVR).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil {
			if k8errors.IsNotFound(err) {
				return nil
			}

			return fmt.Errorf("Failed to delete NodeUser: %w", err)
		}

		return nil
	})
}

func NodeUserExists(ctx context.Context, kubeProvider kubernetes.KubeClientProviderWithCtx, name string) (bool, error) {
	kubeCl, err := kubeProvider.KubeClientCtx(ctx)
	if err != nil {
		return false, err
	}

	var exists bool

	err = retry.NewSilentLoopWithParamsOpts(
		retry.WithName("Check NodeUser %q exists", name),
		retry.WithAttempts(20),
		retry.WithWait(1*time.Second),
	).RunContext(ctx, func() error {
		timeoutCtx, cancel := defaultTimeoutCtx(ctx)
		defer cancel()

		_, err := kubeCl.Dynamic().Resource(v1.NodeUserGVR).Get(timeoutCtx, name, metav1.GetOptions{})
		if err != nil {
			if k8errors.IsNotFound(err) {
				exists = false
				return nil
			}

			return fmt.Errorf("Failed to get NodeUser %q: %w", name, err)
		}

		exists = true
		return nil
	})
	if err != nil {
		return false, err
	}

	return exists, nil
}

type NodeUserPresentsChecker func(node corev1.Node) bool

type NodeUserPresentsWaiter struct {
	params  retry.Params
	checker NodeUserPresentsChecker

	kubeProvider kubernetes.KubeClientProviderWithCtx
}

func NewNodeUserExistsWaiter(checker NodeUserPresentsChecker, kubeProvider kubernetes.KubeClientProviderWithCtx) *NodeUserPresentsWaiter {
	params := retry.NewEmptyParams(
		retry.WithAttempts(250),
		retry.WithWait(1*time.Second),
	)

	return &NodeUserPresentsWaiter{
		params:       params,
		checker:      checker,
		kubeProvider: kubeProvider,
	}
}

func NewConvergerNodeUserExistsWaiter(kubeProvider kubernetes.KubeClientProviderWithCtx) *NodeUserPresentsWaiter {
	return NewNodeUserExistsWaiter(v1.ConvergerNodeUserExistsChecker, kubeProvider)
}

func (w *NodeUserPresentsWaiter) WithParams(params retry.Params) *NodeUserPresentsWaiter {
	// no-op if filled params nil or invalid
	if govalue.IsNil(params) {
		return w
	}

	w.params = w.params.Clone(
		retry.WithName("%s", params.Name()),
		retry.WithAttempts(params.Attempts()),
		retry.WithWait(params.Wait()),
	)

	return w
}

func (w *NodeUserPresentsWaiter) WaitPresentOnNodes(ctx context.Context, nodeUser *v1.NodeUserCredentials) error {
	nodeUserName := nodeUser.Name
	listOpts := metav1.ListOptions{}

	if len(nodeUser.NodeGroups) > 0 {
		selector, err := kubernetes.GetLabelSelector([]kubernetes.LabelSelector{
			{
				Label:    global.NodeGroupLabel,
				Operator: selection.In,
				Vals:     nodeUser.NodeGroups,
			},
		})
		if err != nil {
			return err
		}
		listOpts.LabelSelector = selector
	}

	return retry.NewLoopWithParams(w.loopParams(nodeUserName)).
		RunContext(ctx, func() error {
			kubeCl, err := w.kubeProvider.KubeClientCtx(ctx)
			if err != nil {
				return err
			}

			nodesForClient, err := kubeCl.CoreV1().Nodes().List(ctx, listOpts)
			if err != nil {
				return err
			}

			if len(nodesForClient.Items) == 0 {
				return fmt.Errorf(
					"NodeUser '%s' is not present on nodes yet. No node found for selector '%s'",
					nodeUserName,
					listOpts.LabelSelector,
				)
			}

			notPresentInNodes := make([]string, 0, len(nodesForClient.Items))

			for _, node := range nodesForClient.Items {
				if !w.checker(node) {
					notPresentInNodes = append(notPresentInNodes, node.Name)
				}
			}

			if len(notPresentInNodes) > 0 {
				return fmt.Errorf(
					"NodeUser '%s' is not present on nodes [%s] yet",
					nodeUserName,
					strings.Join(notPresentInNodes, ", "),
				)
			}

			return nil
		})
}

func (w *NodeUserPresentsWaiter) loopParams(userName string) retry.Params {
	return w.params.Clone(retry.WithName("Waiting for NodeUser '%s' present on hosts", userName))
}

func defaultTimeoutCtx(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, 10*time.Second)
}

func createNodeUser(ctx context.Context, kubeCl client.KubeClient, nodeUserResource *unstructured.Unstructured) error {
	timeoutCtx, cancel := defaultTimeoutCtx(ctx)
	defer cancel()

	_, err := kubeCl.Dynamic().Resource(v1.NodeUserGVR).Create(timeoutCtx, nodeUserResource, metav1.CreateOptions{})

	return err
}

func updateNodeUser(ctx context.Context, kubeCl client.KubeClient, nodeUserResource *unstructured.Unstructured) error {
	timeoutCtx, cancel := defaultTimeoutCtx(ctx)
	defer cancel()

	_, err := kubeCl.Dynamic().Resource(v1.NodeUserGVR).Update(timeoutCtx, nodeUserResource, metav1.UpdateOptions{})

	return err
}
