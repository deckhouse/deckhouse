/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package bashiblecontext assembles the bashible-apiserver-context Secret's
// input.yaml from live kube objects, taking over the heavy bashible consumer of
// the get_crds hook. Every field is READ from an already-materialised object
// (Secret/ConfigMap/file) — the lifecycle hooks that own token rotation and cert
// issuance keep running; the node-controller only copies their output. The one
// computed field is clusterMasterEndpoints (no ready Secret exists), which must
// stay byte-parity with discover_apiserver_endpoints.
package bashiblecontext

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ngcommon "github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
)

const (
	cloudInstanceManagerNS = "d8-cloud-instance-manager"
	kubeSystemNS           = "kube-system"

	packagesProxyTokenSecretName = "registry-packages-proxy-token"

	controlPlaneArgsSecretName = "d8-control-plane-manager-control-plane-arguments"

	apiProxyCertSecretName = "kubernetes-api-proxy-discovery-cert"

	cloudProviderSecretName = ngcommon.CloudProviderSecretName

	bootstrapTokenNGLabel = "node-manager.deckhouse.io/node-group"

	// defaultRootCAFile mirrors discover_kubernetes_ca.rootCAFile: the in-pod
	// service-account CA the bashible input.yaml carries as kubernetesCA.
	defaultRootCAFile = "/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

// allowedBundles is the static internal.allowedBundles default
// (openapi/values.yaml). get_crds never computes it; it is a constant.
var allowedBundles = []string{"ubuntu-lts", "centos", "debian", "opensuse"}

// Service reads the bashible input.yaml fields from live kube objects.
type Service struct {
	Client client.Client
	// RootCAFile overrides the service-account CA path (tests set it); empty
	// means defaultRootCAFile.
	RootCAFile string
}

// readCloudProvider mirrors discover_cloud_provider: base64-decoded (by the
// client) Secret values JSON-unmarshalled into the internal.cloudProvider tree.
// Returns nil when the Secret is absent so the caller can omit the field, matching
// the template's `if hasKey internal cloudProvider` guard.
func (s *Service) readCloudProvider(ctx context.Context) map[string]interface{} {
	secret := &corev1.Secret{}
	if err := s.Client.Get(ctx, types.NamespacedName{Namespace: kubeSystemNS, Name: cloudProviderSecretName}, secret); err != nil {
		return nil
	}
	return decodeSecretData(secret.Data)
}

// decodeSecretData decodes each Secret value as JSON, falling back to the raw
// string when it is not valid JSON (corev1.Secret.Data is already base64-decoded).
func decodeSecretData(data map[string][]byte) map[string]interface{} {
	res := make(map[string]interface{}, len(data))
	for k, v := range data {
		var val interface{}
		if err := json.Unmarshal(v, &val); err != nil {
			res[k] = string(v)
			continue
		}
		res[k] = val
	}
	return res
}

// readPackagesProxyToken mirrors get_packages_proxy_token: the "token" value of
// d8-cloud-instance-manager/registry-packages-proxy-token ("" when absent). The
// hook always sets internal.packagesProxy.token, so the field is always emitted.
func (s *Service) readPackagesProxyToken(ctx context.Context) string {
	secret := &corev1.Secret{}
	if err := s.Client.Get(ctx, types.NamespacedName{Namespace: cloudInstanceManagerNS, Name: packagesProxyTokenSecretName}, secret); err != nil {
		return ""
	}
	return string(secret.Data["token"])
}

// controlPlaneArguments carries the two input.yaml fields derived from
// control_plane_arguments; present is false when the source Secret is absent, in
// which case both fields are omitted (matching the hook's Remove).
type controlPlaneArguments struct {
	present bool
	// updateFrequency is nil when nodeMonitorGracePeriod is 0 (field omitted).
	updateFrequency    *float64
	kubeletFeatureGate []string
}

type nodeArguments struct {
	NodeMonitorGracePeriodSeconds int64 `json:"nodeMonitorGracePeriod,omitempty"`
}

type featureGatesData struct {
	Kubelet []string `json:"kubelet,omitempty"`
}

// readControlPlaneArguments mirrors control_plane_arguments: nodeStatusUpdate-
// Frequency = round(nodeMonitorGracePeriod/4) (omitted when 0) and allowed-
// KubeletFeatureGates = kubelet feature gates ([] when the key is absent but the
// Secret exists).
func (s *Service) readControlPlaneArguments(ctx context.Context) controlPlaneArguments {
	secret := &corev1.Secret{}
	if err := s.Client.Get(ctx, types.NamespacedName{Namespace: kubeSystemNS, Name: controlPlaneArgsSecretName}, secret); err != nil {
		return controlPlaneArguments{}
	}

	res := controlPlaneArguments{present: true, kubeletFeatureGate: []string{}}

	if argData, ok := secret.Data["arguments.json"]; ok {
		var args nodeArguments
		if err := json.Unmarshal(argData, &args); err == nil && args.NodeMonitorGracePeriodSeconds != 0 {
			freq := math.Round(float64(args.NodeMonitorGracePeriodSeconds) / 4)
			res.updateFrequency = &freq
		}
	}

	if fgData, ok := secret.Data["featureGates.json"]; ok {
		var fg featureGatesData
		if err := json.Unmarshal(fgData, &fg); err == nil && fg.Kubelet != nil {
			res.kubeletFeatureGate = fg.Kubelet
		}
	}

	return res
}

// apiserverProxyCerts carries the discovery cert/key; present is false when the
// Secret is absent so the caller omits apiserverProxyCerts entirely.
type apiserverProxyCerts struct {
	present bool
	crt     string
	key     string
}

// readAPIServerProxyCerts reads the materialised kube-system/kubernetes-api-proxy-
// discovery-cert Secret directly (crt/key keys), rather than the hook's snapshot
// value which is only re-set on rotation.
func (s *Service) readAPIServerProxyCerts(ctx context.Context) apiserverProxyCerts {
	secret := &corev1.Secret{}
	if err := s.Client.Get(ctx, types.NamespacedName{Namespace: kubeSystemNS, Name: apiProxyCertSecretName}, secret); err != nil {
		return apiserverProxyCerts{}
	}
	return apiserverProxyCerts{
		present: true,
		crt:     string(secret.Data["crt"]),
		key:     string(secret.Data["key"]),
	}
}

// readKubernetesCA mirrors discover_kubernetes_ca: the in-pod service-account CA
// file ("" when unreadable, so the template's truthiness guard omits it).
func (s *Service) readKubernetesCA() string {
	path := s.RootCAFile
	if path == "" {
		path = defaultRootCAFile
	}
	caBytes, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(caBytes)
}

// readBootstrapTokens mirrors order_bootstrap_token's value side: for every
// NodeGroup-labelled bootstrap-token Secret in kube-system it keeps the newest
// non-expired token per NodeGroup as "<id>.<secret>". The hook still owns
// creation/rotation; this only reproduces the resulting internal.bootstrapTokens
// map from the Secrets already in the cluster.
func (s *Service) readBootstrapTokens(ctx context.Context) map[string]string {
	req, err := labels.NewRequirement(bootstrapTokenNGLabel, selection.Exists, nil)
	if err != nil {
		return map[string]string{}
	}
	secrets := &corev1.SecretList{}
	if err := s.Client.List(ctx, secrets,
		client.InNamespace(kubeSystemNS),
		client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(*req)},
	); err != nil {
		return map[string]string{}
	}

	type candidate struct {
		token   string
		created time.Time
	}
	newest := make(map[string]candidate)

	for i := range secrets.Items {
		sec := &secrets.Items[i]
		if sec.Type != corev1.SecretTypeBootstrapToken {
			continue
		}
		ng := sec.Labels[bootstrapTokenNGLabel]
		if ng == "" {
			continue
		}

		if raw, ok := sec.Data["expiration"]; ok {
			expire, err := time.Parse(time.RFC3339, string(raw))
			if err != nil || time.Until(expire) < 0 {
				continue
			}
		}

		id, hasID := sec.Data["token-id"]
		secretPart, hasSecret := sec.Data["token-secret"]
		if !hasID || !hasSecret {
			continue
		}
		token := string(id) + "." + string(secretPart)

		created := sec.CreationTimestamp.Time
		if cur, ok := newest[ng]; !ok || created.After(cur.created) {
			newest[ng] = candidate{token: token, created: created}
		}
	}

	res := make(map[string]string, len(newest))
	for ng, c := range newest {
		res[ng] = c.token
	}
	return res
}
