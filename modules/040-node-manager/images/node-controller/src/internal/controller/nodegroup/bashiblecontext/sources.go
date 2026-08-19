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

package bashiblecontext

import (
	"context"
	"encoding/json"
	"math"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nodecommon "github.com/deckhouse/node-controller/internal/common"
	ngcommon "github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
)

const (
	cloudInstanceManagerNS = "d8-cloud-instance-manager"
	kubeSystemNS           = "kube-system"

	packagesProxyTokenSecretName = "registry-packages-proxy-token"

	controlPlaneArgsSecretName = "d8-control-plane-manager-control-plane-arguments"

	apiProxyCertSecretName = "kubernetes-api-proxy-discovery-cert"

	cloudProviderSecretName = ngcommon.CloudProviderSecretName
)

// rootCAFiles are the candidate locations of the projected service-account CA, canonical path
// first. See readKubernetesCA.
var rootCAFiles = []string{
	"/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
	"/run/secrets/kubernetes.io/serviceaccount/ca.crt",
}

// allowedBundles is the same in every edition: it mirrors the distributions declared in
// candi/version_map.yml (shared by all editions) and the allowedBundles default in every
// openapi/values.yaml. Keep the three in sync — the values list also drives the bashible
// Role's resourceNames, so a shorter list here silently disagrees with the granted RBAC.
var allowedBundles = []string{"ubuntu-lts", "centos", "debian", "redos", "rosa", "astra", "altlinux", "opensuse"}

// Service reads the bashible input.yaml fields from live kube objects.
type Service struct {
	Client     client.Client
	Reader     client.Reader
	RootCAFile string
}

func (s *Service) reader() client.Reader {
	if s.Reader != nil {
		return s.Reader
	}
	return s.Client
}

func (s *Service) readCloudProvider(ctx context.Context) map[string]interface{} {
	secret := &corev1.Secret{}
	if err := s.Client.Get(ctx, types.NamespacedName{Namespace: kubeSystemNS, Name: cloudProviderSecretName}, secret); err != nil {
		return nil
	}
	return decodeSecretData(secret.Data)
}

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

func (s *Service) readPackagesProxyToken(ctx context.Context) string {
	secret := &corev1.Secret{}
	if err := s.reader().Get(ctx, types.NamespacedName{Namespace: cloudInstanceManagerNS, Name: packagesProxyTokenSecretName}, secret); err != nil {
		return ""
	}
	return string(secret.Data["token"])
}

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

func (s *Service) readControlPlaneArguments(ctx context.Context) controlPlaneArguments {
	secret := &corev1.Secret{}
	if err := s.reader().Get(ctx, types.NamespacedName{Namespace: kubeSystemNS, Name: controlPlaneArgsSecretName}, secret); err != nil {
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

type apiserverProxyCerts struct {
	present bool
	crt     string
	key     string
}

func (s *Service) readAPIServerProxyCerts(ctx context.Context) apiserverProxyCerts {
	secret := &corev1.Secret{}
	if err := s.reader().Get(ctx, types.NamespacedName{Namespace: kubeSystemNS, Name: apiProxyCertSecretName}, secret); err != nil {
		return apiserverProxyCerts{}
	}
	return apiserverProxyCerts{
		present: true,
		crt:     string(secret.Data["crt"]),
		key:     string(secret.Data["key"]),
	}
}

// readKubernetesCA reads the projected service-account CA. The kubelet mounts it under
// /var/run/..., which resolves to /run/... only in images where /var/run is a symlink — the
// hook this was ported from ran in the deckhouse image (where it is), node-controller runs on
// distroless. Both spellings are therefore tried, so the CA never silently ends up empty in the
// bashible context.
func (s *Service) readKubernetesCA() string {
	paths := rootCAFiles
	if s.RootCAFile != "" {
		paths = []string{s.RootCAFile}
	}
	for _, path := range paths {
		caBytes, err := os.ReadFile(path)
		if err == nil {
			return string(caBytes)
		}
	}
	return ""
}

// readBootstrapTokens keeps the Service's read-or-nothing style: input.yaml is
// rendered from whatever could be read, and a missing token is a missing key.
func (s *Service) readBootstrapTokens(ctx context.Context) map[string]string {
	tokens, err := nodecommon.BootstrapTokens(ctx, s.reader())
	if err != nil {
		return map[string]string{}
	}
	return tokens
}
