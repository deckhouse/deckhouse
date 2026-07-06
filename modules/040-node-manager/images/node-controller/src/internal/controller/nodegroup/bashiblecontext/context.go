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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"
)

const (
	// secretName/secretNamespace/secretInputKey identify the Secret the helm
	// define bashible_input_data currently renders; the controller becomes its
	// single writer at cutover (both must never write it at the same time).
	secretName      = "bashible-apiserver-context"
	secretNamespace = "d8-cloud-instance-manager"
	secretInputKey  = "input.yaml"
)

// Globals carries the global.*/deckhouse.* input.yaml fields. ReadGlobals
// assembles it from live kube objects (the version-info ConfigMap, the
// cluster-configuration Secret, the cluster-uuid ConfigMap and the DNS Service),
// so a standalone pod re-derives the values the helm path resolves from global
// values. clusterDomain/clusterDNSAddress are emitted verbatim; Proxy, when
// non-nil, is passed through as the proxy block.
type Globals struct {
	DeckhouseChannel        string
	DeckhouseVersion        string
	DeckhouseEdition        string
	PodSubnetNodeCIDRPrefix string
	ClusterDomain           string
	ClusterDNSAddress       string
	ClusterUUID             string
	// Proxy is the resolved proxy block (httpProxy/httpsProxy/noProxy) or nil
	// when the cluster has no proxy configured.
	Proxy map[string]interface{}
}

// Build assembles the bashible input.yaml value tree, mirroring the helm define
// bashible_input_data (templates/bashible-apiserver/deployment.yaml). nodeGroups
// is the internal.nodeGroups blob (one BuildNodeGroupBlob element per NodeGroup)
// the node-controller now owns; every other field is read from live kube objects
// or carried in globals. Optional blocks are omitted exactly where the template
// gates on hasKey/truthiness, so the produced YAML is value-equivalent to the
// helm render and keeps the bashible bootstrap checksum stable.
func (s *Service) Build(ctx context.Context, globals Globals, nodeGroups []map[string]interface{}) map[string]interface{} {
	cpArgs := s.readControlPlaneArguments(ctx)
	certs := s.readAPIServerProxyCerts(ctx)
	eps := s.readEndpoints(ctx)

	input := map[string]interface{}{
		"deckhouse": map[string]interface{}{
			"channel": defaultString(globals.DeckhouseChannel, "unknown"),
			"version": globals.DeckhouseVersion,
			"edition": globals.DeckhouseEdition,
		},
		"podSubnetNodeCIDRPrefix": globals.PodSubnetNodeCIDRPrefix,
		"clusterDomain":           globals.ClusterDomain,
		"clusterDNSAddress":       globals.ClusterDNSAddress,
		"clusterUUID":             defaultString(globals.ClusterUUID, "00000000-0000-0000-0000-000000000000"),
		"bootstrapTokens":         s.readBootstrapTokens(ctx),
		"apiserverEndpoints":      eps.apiserverEndpoints,
		"clusterMasterEndpoints":  eps.clusterMasterEndpoints,
		"packagesProxy": map[string]interface{}{
			"token": s.readPackagesProxyToken(ctx),
		},
		"allowedBundles": allowedBundles,
		"nodeGroups":     nodeGroups,
	}

	if cp := s.readCloudProvider(ctx); cp != nil {
		input["cloudProvider"] = cp
	}
	if globals.Proxy != nil {
		input["proxy"] = globals.Proxy
	}
	if certs.present {
		input["apiserverProxyCerts"] = map[string]interface{}{
			"crt": certs.crt,
			"key": certs.key,
		}
	}
	if ca := s.readKubernetesCA(); ca != "" {
		input["kubernetesCA"] = ca
	}
	if cpArgs.present {
		if cpArgs.updateFrequency != nil {
			input["nodeStatusUpdateFrequency"] = *cpArgs.updateFrequency
		}
		input["allowedKubeletFeatureGates"] = cpArgs.kubeletFeatureGate
	}

	return input
}

// Marshal renders the input.yaml payload the same way the helm path stores it
// (sigs.k8s.io/yaml, i.e. JSON-tagged marshalling with sorted keys). This is the
// exact string written under the Secret's input.yaml key.
func Marshal(input map[string]interface{}) ([]byte, error) {
	return yaml.Marshal(input)
}

// WriteSecret assembles input.yaml and upserts the bashible-apiserver-context
// Secret — the single-writer counterpart to the helm define bashible_input_data.
// Globals are read from kube here; the caller is the completeness gate for the
// nodeGroups blob, which must contain every NodeGroup since a partial input.yaml
// breaks bashible-apiserver bootstrap. The standard module labels are set so the
// Secret is managed/selected exactly like the helm-rendered one.
func (s *Service) WriteSecret(ctx context.Context, nodeGroups []map[string]interface{}) error {
	raw, err := Marshal(s.Build(ctx, s.ReadGlobals(ctx), nodeGroups))
	if err != nil {
		return fmt.Errorf("marshal input.yaml: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: secretNamespace},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, s.Client, secret, func() error {
		if secret.Labels == nil {
			secret.Labels = map[string]string{}
		}
		secret.Labels["heritage"] = "deckhouse"
		secret.Labels["module"] = "node-manager"
		secret.Labels["app"] = "bashible-apiserver"
		secret.Data = map[string][]byte{secretInputKey: raw}
		return nil
	})
	if err != nil {
		return fmt.Errorf("upsert %s/%s: %w", secretNamespace, secretName, err)
	}
	return nil
}

func defaultString(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
