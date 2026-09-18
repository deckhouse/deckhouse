// Copyright 2026 Flant JSC
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

package rpp

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

const (
	// The in-cluster half of registry packages proxy: the only thing that ever publishes the
	// rpp-get bootstrap port on a master's LAN address. dhctl's own bootstrap server reaches
	// the master through a reverse tunnel bound to 127.0.0.1 there, which no other machine
	// can dial.
	inClusterProxyNamespace  = "d8-cloud-instance-manager"
	inClusterProxyDeployment = "registry-packages-proxy"

	// Ten minutes: room for a slow registry, and no more. The budget only bites where the module
	// is not coming up at all, and a longer one buys nothing there - machines built against a
	// proxy that never answers die on their own 150-second timer regardless.
	inClusterProxyReadyAttempts = 120
	inClusterProxyReadyInterval = 5 * time.Second
)

// WaitForInClusterProxy blocks until registry-packages-proxy has a ready replica, which is what
// publishes the rpp-get bootstrap port on the masters' own addresses.
//
// Every machine dhctl creates from the cloud provider downloads rpp-get from that port before it
// can run anything else, and its cloud-init gives up after 30 attempts five seconds apart. Built
// against a proxy that is not up, a machine burns that budget and stays empty for good.
//
// A precondition check rather than a fix for a race: the ordinary bootstrap path has already
// waited for Deckhouse by the time it runs. See the call site for the path it covers, and for
// what it deliberately does not prove.
func WaitForInClusterProxy(ctx context.Context, kubeCl client.KubeClient) error {
	return waitForInClusterProxy(ctx, kubeCl, inClusterProxyReadyAttempts, inClusterProxyReadyInterval)
}

func waitForInClusterProxy(ctx context.Context, kubeCl client.KubeClient, attempts int, interval time.Duration) error {
	name := fmt.Sprintf("%s/%s", inClusterProxyNamespace, inClusterProxyDeployment)

	return retry.NewLoop(fmt.Sprintf("Waiting for %s to serve the rpp-get bootstrap port", name), attempts, interval).
		RunContext(ctx, func() error {
			deployment, err := kubeCl.AppsV1().Deployments(inClusterProxyNamespace).Get(ctx, inClusterProxyDeployment, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					return fmt.Errorf("Deployment %s does not exist yet: the registry-packages-proxy module has not rolled out", name)
				}

				return fmt.Errorf("get Deployment %s: %w", name, err)
			}

			// One ready replica is the whole condition, not the desired count: on an HA
			// cluster the Deployment asks for a replica per master while dhctl has built
			// only the first one, and the machines this wait guards are what brings the
			// rest of them up.
			if deployment.Status.ReadyReplicas < 1 {
				return fmt.Errorf("Deployment %s has no ready replica yet (%d of %d)",
					name, deployment.Status.ReadyReplicas, deployment.Status.Replicas)
			}

			return nil
		})
}
