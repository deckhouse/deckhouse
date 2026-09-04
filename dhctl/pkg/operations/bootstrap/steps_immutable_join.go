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
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	libretry "github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
	kubeapiserver "github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/apiserver"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

// buildImmutableJoinPayload renders the cloud-init an additional master joins the
// running cluster with: the CA, the current bootstrap token, and the live apiservers
// are read from it; all else matches master 0. No ControlPlaneConfig: see immutable.BuildJoinPayload.
func buildImmutableJoinPayload(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	metaConfig *config.MetaConfig,
	nodeName string,
) (string, error) {
	var (
		caCert    string
		token     string
		endpoints []string
	)

	// Retried as one: the three reads are the payload's only inputs from the
	// running cluster, and each of them is published asynchronously.
	err := libretry.NewLoop(fmt.Sprintf("Waiting for the cluster to publish what %s joins with", nodeName),
		waitJoinInputs.attempts, waitJoinInputs.interval).
		RunContext(ctx, func() error {
			var err error
			caCert, err = clusterCABase64(ctx, kubeCl)
			if err != nil {
				return err
			}
			token, err = groupBootstrapToken(ctx, kubeCl, global.MasterNodeGroupName)
			if err != nil {
				return err
			}
			endpoints, err = apiServerEndpoints(ctx, kubeCl)
			return err
		})
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
	cm, err := kubeCl.CoreV1().ConfigMaps(global.ConfigsNS).Get(ctx, clusterCAConfigMap, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("read the cluster CA from %s/%s: %w", global.ConfigsNS, clusterCAConfigMap, err)
	}
	ca := cm.Data[clusterCAKey]
	if ca == "" {
		return "", fmt.Errorf("configmap %s/%s carries no %s", global.ConfigsNS, clusterCAConfigMap, clusterCAKey)
	}
	return base64.StdEncoding.EncodeToString([]byte(ca)), nil
}

// groupBootstrapToken returns the group's newest non-expired bootstrap token, so a
// node cannot boot with one that expires mid-install.
// Mirrors readBootstrapToken of modules/040-node-manager/images/node-controller/src/internal/controller/nodebootstrap/render.go.
func groupBootstrapToken(ctx context.Context, kubeCl *client.KubernetesClient, ngName string) (string, error) {
	secrets, err := kubeCl.CoreV1().Secrets(global.ConfigsNS).List(ctx, metav1.ListOptions{
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

// apiServerEndpoints are the apiservers a joining node talks to until it runs one
// itself. Derived from the cluster, not from the manual-bootstrap-for-master secret,
// which is published too late.
func apiServerEndpoints(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
) ([]string, error) {
	endpoints, err := kubeapiserver.GetEndpoints(ctx, kubeCl)
	if err != nil {
		return nil, err
	}

	urls := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		urls = append(urls, "https://"+endpoint)
	}

	return urls, nil
}
