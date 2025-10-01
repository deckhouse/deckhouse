// Copyright 2022 Flant JSC
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

package bootstrap

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

func CheckPreventBreakAnotherBootstrappedCluster(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	config *config.DeckhouseInstaller,
) error {
	return retry.NewSilentLoop("Check prevent break another bootstrapped", 15, 3*time.Second).RunContext(ctx, func() error {
		var uuidInCluster string
		cmInCluster, err := kubeCl.CoreV1().ConfigMaps(manifests.ClusterUUIDCmNamespace).Get(ctx, manifests.ClusterUUIDCm, metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}

		if err == nil {
			uuidInCluster = cmInCluster.Data[manifests.ClusterUUIDCmKey]
			if uuidInCluster == "" {
				return fmt.Errorf("Cluster UUID config map found, but UUID is empty")
			}
		}

		if uuidInCluster == "" {
			return nil
		}

		if uuidInCluster != config.UUID {
			return fmt.Errorf(`Cluster UUID's not equal in the cluster (%s) and in the cache (%s).
Probably you are trying bootstrap cluster on node with previous created cluster.
Please check hostname.`, uuidInCluster, config.UUID)
		}

		return nil
	})
}

func WaitForFirstMasterNodeBecomeReady(ctx context.Context, kubeCl *client.KubernetesClient) error {
	var nodeName string
	err := retry.NewSilentLoop("Get master node name", 45, 3*time.Second).RunContext(ctx, func() error {
		nodes, err := kubeCl.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}

		if len(nodes.Items) == 0 {
			return fmt.Errorf("no master node found")
		}

		nodeName = nodes.Items[0].Name

		return nil
	})
	if err != nil {
		return err
	}
	return entity.WaitForSingleNodeBecomeReady(ctx, kubeCl, nodeName)
}
