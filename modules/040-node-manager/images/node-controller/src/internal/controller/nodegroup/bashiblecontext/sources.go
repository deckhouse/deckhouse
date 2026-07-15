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

	defaultRootCAFile = "/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

var allowedBundles = []string{"ubuntu-lts", "centos", "debian", "opensuse"}

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

func (s *Service) readBootstrapTokens(ctx context.Context) map[string]string {
	req, err := labels.NewRequirement(bootstrapTokenNGLabel, selection.Exists, nil)
	if err != nil {
		return map[string]string{}
	}
	secrets := &corev1.SecretList{}
	if err := s.reader().List(ctx, secrets,
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
