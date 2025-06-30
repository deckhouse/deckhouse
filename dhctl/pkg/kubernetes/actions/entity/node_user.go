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
	"time"

	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sdk "github.com/deckhouse/module-sdk/pkg/utils"

	v1 "github.com/deckhouse/deckhouse/dhctl/pkg/apis/v1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

func CreateNodeUser(ctx context.Context, kubeGetter kubernetes.KubeClientProvider, nodeUser *v1.NodeUser) error {
	nodeUserResource, err := sdk.ToUnstructured(nodeUser)
	if err != nil {
		return fmt.Errorf("failed to convert NodeUser to unstructured: %w", err)
	}

	return retry.NewLoop("Save dhctl converge NodeUser", 45, 10*time.Second).RunContext(ctx, func() error {
		_, err = kubeGetter.KubeClient().Dynamic().Resource(v1.NodeUserGVK).Create(ctx, nodeUserResource, metav1.CreateOptions{})

		if err != nil {
			if k8errors.IsAlreadyExists(err) {
				_, err = kubeGetter.KubeClient().Dynamic().Resource(v1.NodeUserGVK).Update(ctx, nodeUserResource, metav1.UpdateOptions{})
				return err
			}

			return fmt.Errorf("failed to create NodeUser: %w", err)
		}

		return nil
	})
}

func DeleteNodeUser(ctx context.Context, kubeGetter kubernetes.KubeClientProvider, name string) error {
	return retry.NewLoop("Delete dhctl converge NodeUser", 45, 10*time.Second).RunContext(ctx, func() (err error) {
		err = kubeGetter.KubeClient().Dynamic().Resource(v1.NodeUserGVK).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil {
			if k8errors.IsNotFound(err) {
				return nil
			}

			return fmt.Errorf("failed to delete NodeUser: %w", err)
		}

		return nil
	})
}
