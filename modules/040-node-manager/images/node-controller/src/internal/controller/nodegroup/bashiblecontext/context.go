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
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

const (
	secretName      = "bashible-apiserver-context"
	secretNamespace = "d8-cloud-instance-manager"
	secretInputKey  = "input.yaml"
)

type Globals struct {
	DeckhouseChannel        string
	DeckhouseVersion        string
	DeckhouseEdition        string
	PodSubnetNodeCIDRPrefix string
	ClusterDomain           string
	ClusterDNSAddress       string
	ClusterUUID             string
	Proxy                   map[string]interface{}
}

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

func Marshal(input map[string]interface{}) ([]byte, error) {
	return yaml.Marshal(input)
}

func (s *Service) WriteSecret(ctx context.Context, nodeGroups []map[string]interface{}) error {
	logger := log.FromContext(ctx)

	globals := s.ReadGlobals(ctx)
	raw, err := Marshal(s.Build(ctx, globals, nodeGroups))
	if err != nil {
		return fmt.Errorf("marshal input.yaml: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: secretNamespace},
	}
	op, err := controllerutil.CreateOrUpdate(ctx, s.Client, secret, func() error {
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

	ngVersions := make(map[string]interface{}, len(nodeGroups))
	for _, ng := range nodeGroups {
		name, _ := ng["name"].(string)
		if name == "" {
			continue
		}
		ngVersions[name] = ng["kubernetesVersion"]
	}
	logger.Info("wrote bashible-apiserver-context Secret",
		"secret", secretNamespace+"/"+secretName,
		"operation", op,
		"bytes", len(raw),
		"nodeGroupCount", len(nodeGroups),
		"nodeGroupVersions", ngVersions,
		"clusterDomain", globals.ClusterDomain,
		"podSubnetNodeCIDRPrefix", globals.PodSubnetNodeCIDRPrefix,
		"clusterDNSAddress", globals.ClusterDNSAddress,
		"clusterUUID", globals.ClusterUUID,
	)
	return nil
}

func defaultString(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
