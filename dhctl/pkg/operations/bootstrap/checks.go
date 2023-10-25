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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

func CheckPreventBreakAnotherBootstrappedCluster(kubeCl *client.KubernetesClient, config *config.DeckhouseInstaller) error {
	var uuidInCluster string
	cmInCluster, err := kubeCl.CoreV1().ConfigMaps(manifests.ClusterUUIDCmNamespace).Get(context.TODO(), manifests.ClusterUUIDCm, metav1.GetOptions{})
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
}
