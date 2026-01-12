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

package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeleteNodeObjectFromCluster(ctx context.Context, kubeCl *client.KubernetesClient, nodeName string) error {
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
