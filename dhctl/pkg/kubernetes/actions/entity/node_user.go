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

	sdk "github.com/deckhouse/module-sdk/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	v1 "github.com/deckhouse/deckhouse/dhctl/pkg/apis/v1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

func CreateNodeUser(ctx context.Context, kubeProvider kubernetes.KubeClientProviderWithCtx, nodeUser *v1.NodeUser) error {
	nodeUserResource, err := sdk.ToUnstructured(nodeUser)
	if err != nil {
		return fmt.Errorf("Failed to convert NodeUser to unstructured: %w", err)
	}

	return retry.NewLoop("Save dhctl converge NodeUser", 45, 10*time.Second).RunContext(ctx, func() error {
		kubeCl, err := kubeProvider.KubeClientCtx(ctx)
		if err != nil {
			return err
		}

		_, err = kubeCl.Dynamic().Resource(v1.NodeUserGVK).Create(ctx, nodeUserResource, metav1.CreateOptions{})
		if err != nil {
			if k8errors.IsAlreadyExists(err) {
				_, err = kubeCl.Dynamic().Resource(v1.NodeUserGVK).Update(ctx, nodeUserResource, metav1.UpdateOptions{})
				return err
			}

			return fmt.Errorf("Failed to create NodeUser: %w", err)
		}

		return nil
	})
}

func DeleteNodeUser(ctx context.Context, kubeProvider kubernetes.KubeClientProviderWithCtx, name string) error {
	return retry.NewLoop("Delete dhctl converge NodeUser", 45, 10*time.Second).RunContext(ctx, func() (err error) {
		kubeCl, err := kubeProvider.KubeClientCtx(ctx)
		if err != nil {
			return err
		}
		err = kubeCl.Dynamic().Resource(v1.NodeUserGVK).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil {
			if k8errors.IsNotFound(err) {
				return nil
			}

			return fmt.Errorf("Failed to delete NodeUser: %w", err)
		}

		return nil
	})
}

type NodeUserPresentsChecker func(node corev1.Node) bool

type NodeUserPresentsWaiter struct {
	attempts int
	sleep    time.Duration
	checker  NodeUserPresentsChecker

	kubeProvider kubernetes.KubeClientProviderWithCtx
}

func NewNodeUserExistsWaiter(checker NodeUserPresentsChecker, kubeProvider kubernetes.KubeClientProviderWithCtx) *NodeUserPresentsWaiter {
	return &NodeUserPresentsWaiter{
		attempts:     30,
		sleep:        5 * time.Second,
		checker:      checker,
		kubeProvider: kubeProvider,
	}
}

func NewConvergerNodeUserExistsWaiter(kubeProvider kubernetes.KubeClientProviderWithCtx) *NodeUserPresentsWaiter {
	return &NodeUserPresentsWaiter{
		attempts:     30,
		sleep:        5 * time.Second,
		checker:      v1.ConvergerNodeUserExistsChecker,
		kubeProvider: kubeProvider,
	}
}

func (w *NodeUserPresentsWaiter) WaitPresentOnNodes(ctx context.Context, nodeUser *v1.NodeUserCredentials) error {
	nodeUserName := nodeUser.Name
	listOpts := metav1.ListOptions{}

	if len(nodeUser.NodeGroups) > 0 {
		selector := labels.NewSelector()
		r, err := labels.NewRequirement("node.deckhouse.io/group", selection.In, nodeUser.NodeGroups)
		if err != nil {
			return err
		}
		selector = selector.Add(*r)
		listOpts.LabelSelector = selector.String()
	}

	return retry.NewLoop(fmt.Sprintf("Waiting for NodeUser '%s' present on hosts", nodeUserName), w.attempts, w.sleep).
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
					"NodeUser '%s' is not present on nodes yet. No any node found for selector '%s'",
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
