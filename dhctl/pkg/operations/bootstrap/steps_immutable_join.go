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

package bootstrap

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/deckhouse/lib-dhctl/pkg/retry"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

const (
	kubeSystemNS           = "kube-system"
	cloudInstanceManagerNS = "d8-cloud-instance-manager"

	// bootstrapSecretAttempts is how long the join waits for node-manager to
	// publish the master bootstrap secret, one attempt a second. The same
	// bound the bashible path uses for the same secret.
	bootstrapSecretAttempts = 225

	// clusterCAConfigMap carries the cluster CA every ServiceAccount is given.
	// node-controller renders day-2 configs from the same source, so a node
	// bootstrapped here and the same node reconciled later see one CA.
	clusterCAConfigMap = "kube-root-ca.crt"
	clusterCAKey       = "ca.crt"

	// bootstrapTokenNGLabel labels a bootstrap-token secret with the NodeGroup
	// it belongs to.
	bootstrapTokenNGLabel = "node-manager.deckhouse.io/node-group"
)

// buildImmutableJoinPayload renders the cloud-init an additional master boots
// with.
//
// The first master's payload is rendered before anything exists and tells the
// node to create a cluster. This one is rendered against a cluster that already
// runs, and tells the node to join it. Three of its inputs can only come from
// there — the CA that cluster issued, the group's current bootstrap token, and
// the apiservers already serving — so they are read, not rendered. Everything
// else comes from the same installer inputs as master 0, which is what keeps
// the three masters identical in everything except what they join.
//
// No ControlPlaneConfig: see immutable.BuildJoinPayload.
func buildImmutableJoinPayload(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	metaConfig *config.MetaConfig,
	nodeName string,
) (string, error) {
	caCert, err := clusterCABase64(ctx, kubeCl)
	if err != nil {
		return "", err
	}

	token, err := groupBootstrapToken(ctx, kubeCl, global.MasterNodeGroupName)
	if err != nil {
		return "", err
	}

	endpoints, err := apiServerEndpoints(ctx, kubeCl)
	if err != nil {
		return "", err
	}

	return immutable.BuildJoinPayload(ctx, immutable.JoinPayloadInput{
		NodeName:           nodeName,
		MetaConfig:         metaConfig,
		CACert:             caCert,
		BootstrapToken:     token,
		APIServerEndpoints: endpoints,
	})
}

// clusterCABase64 reads the cluster CA the way the on-node agent expects it:
// base64 of the PEM.
func clusterCABase64(ctx context.Context, kubeCl *client.KubernetesClient) (string, error) {
	cm, err := kubeCl.CoreV1().ConfigMaps(kubeSystemNS).Get(ctx, clusterCAConfigMap, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("read the cluster CA from %s/%s: %w", kubeSystemNS, clusterCAConfigMap, err)
	}
	ca := cm.Data[clusterCAKey]
	if ca == "" {
		return "", fmt.Errorf("configmap %s/%s carries no %s", kubeSystemNS, clusterCAConfigMap, clusterCAKey)
	}
	return base64.StdEncoding.EncodeToString([]byte(ca)), nil
}

// groupBootstrapToken returns the newest non-expired bootstrap token of the
// group — the same rotating token a bashible node is given. node-manager keeps
// one per NodeGroup and replaces it as it ages; taking the newest is what keeps
// a node from booting with one that expires while it is still installing.
// Kept in step with readBootstrapToken in the node-controller's
// internal/controller/nodebootstrap/render.go, which picks the token for a
// provisioned machine the same way.
func groupBootstrapToken(ctx context.Context, kubeCl *client.KubernetesClient, ngName string) (string, error) {
	secrets, err := kubeCl.CoreV1().Secrets(kubeSystemNS).List(ctx, metav1.ListOptions{
		LabelSelector: bootstrapTokenNGLabel + "=" + ngName,
	})
	if err != nil {
		return "", fmt.Errorf("list bootstrap tokens of %s: %w", ngName, err)
	}

	token, newest := "", time.Time{}
	for i := range secrets.Items {
		secret := &secrets.Items[i]
		if secret.Type != corev1.SecretTypeBootstrapToken {
			continue
		}
		if raw, ok := secret.Data["expiration"]; ok {
			expire, err := time.Parse(time.RFC3339, string(raw))
			if err != nil || time.Until(expire) < 0 {
				continue
			}
		}
		id, hasID := secret.Data["token-id"]
		value, hasValue := secret.Data["token-secret"]
		if !hasID || !hasValue {
			continue
		}
		if token == "" || secret.CreationTimestamp.After(newest) {
			token = string(id) + "." + string(value)
			newest = secret.CreationTimestamp.Time
		}
	}
	if token == "" {
		return "", fmt.Errorf("no valid bootstrap token for NodeGroup %s", ngName)
	}
	return token, nil
}

// apiServerEndpoints are the apiservers a joining node talks to until it runs
// one itself. They are read from the secret node-manager publishes beside the
// group's cloud config, which is the same list every other node is given — and
// the reason a joining master needs it at all is that it cannot use the first
// master's placeholder, which expands to the node's own address.
//
// The secret is waited for rather than read once: node-manager publishes it
// some time after Deckhouse becomes Ready, and the join starts as soon as the
// first master is. Measured — a bootstrap died on "secrets
// \"manual-bootstrap-for-master\" not found" a tenth of a second after the
// install finished. The bashible path has always waited here for the same
// reason (GetCloudConfig in pkg/kubernetes/actions/entity/node.go), and the
// bound matches its: the endpoints list is what the node cannot boot without,
// so giving up early is worse than waiting.
func apiServerEndpoints(ctx context.Context, kubeCl *client.KubernetesClient) ([]string, error) {
	var secret *corev1.Secret
	err := retry.NewSilentLoop("waiting for the master bootstrap secret", bootstrapSecretAttempts, time.Second).
		RunContext(ctx, func() error {
			got, err := kubeCl.CoreV1().
				Secrets(cloudInstanceManagerNS).
				Get(ctx, "manual-bootstrap-for-"+global.MasterNodeGroupName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			secret = got
			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("read the master bootstrap secret: %w", err)
	}

	var endpoints []string
	if err := yaml.Unmarshal(secret.Data["apiserverEndpoints"], &endpoints); err != nil {
		return nil, fmt.Errorf("parse apiserverEndpoints: %w", err)
	}
	if len(endpoints) == 0 {
		return nil, errors.New("the master bootstrap secret carries no apiserver endpoints")
	}

	// The secret carries host:port; the node's config takes URLs.
	for i, endpoint := range endpoints {
		endpoints[i] = "https://" + endpoint
	}
	return endpoints, nil
}
