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
	"net"
	"sort"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

const (
	kubeSystemNS = "kube-system"

	// The kubernetes Service's EndpointSlice, the cluster's own record of where
	// its apiservers answer. Named the same way node-controller names them.
	apiServerEndpointSliceNS   = "default"
	apiServerEndpointSliceName = "kubernetes"
	apiServerPortName          = "https"

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
// one itself. A joining master cannot use the first master's payload value:
// that one is a placeholder the node expands to its own address.
//
// Derived from the cluster rather than read from node-manager's
// manual-bootstrap-for-master secret, for two reasons. The secret is published
// some time after Deckhouse becomes Ready while the join starts as soon as the
// first master is — measured, a bootstrap died on "secrets
// \"manual-bootstrap-for-master\" not found" a tenth of a second after the
// install finished. And these two sources are what node-controller renders
// spec.apiServerEndpoints from on every pass afterwards
// (readAPIServerEndpoints in the node-controller's
// internal/controller/nodeconfig/sources.go), so a node joins with the value
// its first managed render computes rather than with a different one. Keep the
// two in step. It saves the bump at join, not every bump: once the joining
// master runs an apiserver of its own, its pod joins the list and every
// immutable node's spec gains an address.
func apiServerEndpoints(ctx context.Context, kubeCl *client.KubernetesClient) ([]string, error) {
	set := make(map[string]struct{})

	pods, err := kubeCl.CoreV1().Pods(kubeSystemNS).List(ctx, metav1.ListOptions{
		LabelSelector: "component=kube-apiserver,tier=control-plane",
	})
	if err != nil {
		return nil, fmt.Errorf("list kube-apiserver pods: %w", err)
	}
	for i := range pods.Items {
		pod := &pods.Items[i]
		// A terminating mirror pod keeps its address to the end; a master on
		// its way out must not be handed to a node that is just arriving.
		if pod.DeletionTimestamp != nil || pod.Status.PodIP == "" {
			continue
		}
		set[net.JoinHostPort(pod.Status.PodIP, strconv.Itoa(immutable.APIServerPort))] = struct{}{}
	}

	slice, err := kubeCl.DiscoveryV1().
		EndpointSlices(apiServerEndpointSliceNS).
		Get(ctx, apiServerEndpointSliceName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("read the %s/%s EndpointSlice: %w", apiServerEndpointSliceNS, apiServerEndpointSliceName, err)
	}
	var ports []int32
	for _, port := range slice.Ports {
		if port.Name != nil && *port.Name == apiServerPortName && port.Port != nil {
			ports = append(ports, *port.Port)
		}
	}
	for _, endpoint := range slice.Endpoints {
		for _, addr := range endpoint.Addresses {
			for _, port := range ports {
				set[net.JoinHostPort(addr, strconv.Itoa(int(port)))] = struct{}{}
			}
		}
	}

	if len(set) == 0 {
		return nil, errors.New("the cluster reports no apiserver endpoints")
	}
	endpoints := make([]string, 0, len(set))
	for ep := range set {
		endpoints = append(endpoints, "https://"+ep)
	}
	sort.Strings(endpoints)
	return endpoints, nil
}
