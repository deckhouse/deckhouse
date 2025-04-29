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

package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

func checkAndRestartDeployment(ctx context.Context, kubeClProvider kubernetes.KubeClientProvider, deploymentName string) error {
	hasDeployment := false
	err := retry.NewLoop(fmt.Sprintf("Check deployment %s/%s exists", global.D8SystemNamespace, deploymentName), 10, 5*time.Second).
		RunContext(ctx, func() error {
			kubeCl := kubeClProvider.KubeClient()
			_, err := kubeCl.AppsV1().Deployments(global.D8SystemNamespace).Get(ctx, deploymentName, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					hasDeployment = false
					return nil
				}

				return err
			}

			hasDeployment = true
			return nil
		})

	if err != nil {
		return err
	}

	if !hasDeployment {
		log.InfoF("Deployment %s/%s does not exist. Skip restarting.\n", global.D8SystemNamespace, deploymentName)
		return nil
	}

	err = retry.NewLoop(fmt.Sprintf("Restart deployment %s/%s with adding annotation", global.D8SystemNamespace, deploymentName), 10, 5*time.Second).
		RunContext(ctx, func() error {
			kubeCl := kubeClProvider.KubeClient()
			patch, err := json.Marshal(map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"annotations": map[string]interface{}{
								"dhctl.deckhouse.io/restart-infra-deployment": time.Now().Format(time.RFC3339),
							},
						},
					},
				},
			})
			if err != nil {
				return err
			}

			_, err = kubeCl.AppsV1().Deployments(global.D8SystemNamespace).Patch(
				ctx,
				deploymentName,
				types.MergePatchType,
				patch,
				metav1.PatchOptions{},
			)

			if err != nil {
				return err
			}

			return nil
		})

	if err != nil {
		return err
	}

	return nil
}
