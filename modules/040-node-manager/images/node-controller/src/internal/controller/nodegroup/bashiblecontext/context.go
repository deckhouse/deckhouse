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

	"github.com/deckhouse/node-controller/internal/cloudprovider"
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

func (s *Service) Build(ctx context.Context, globals Globals, nodeGroups []map[string]interface{}, providers cloudprovider.Providers) (map[string]interface{}, error) {
	cpArgs := s.readControlPlaneArguments(ctx)
	certs := s.readAPIServerProxyCerts(ctx)
	eps, err := s.readEndpoints(ctx)
	if err != nil {
		return nil, fmt.Errorf("read kube-apiserver endpoints: %w", err)
	}

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

	all := providers.All()
	if len(all) > 0 {
		trees := make([]map[string]interface{}, 0, len(all))
		for _, p := range all {
			trees = append(trees, p.Data)
		}
		input["cloudProviders"] = trees
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

	return input, nil
}

func Marshal(input map[string]interface{}) ([]byte, error) {
	return yaml.Marshal(input)
}

func (s *Service) WriteSecret(ctx context.Context, nodeGroups []map[string]interface{}, providers cloudprovider.Providers) error {
	logger := log.FromContext(ctx)

	globals := s.ReadGlobals(ctx)
	// The DNS address goes straight into the kubelet config the nodes render
	// (064_configure_kubelet.sh.tpl: "clusterDNS:\n- {{ .normal.clusterDNSAddress }}"), so an
	// empty one produces an invalid entry on every node. Publishing nothing is better than
	// publishing that: the Secret keeps its previous, valid content and the reconcile retries.
	if globals.ClusterDNSAddress == "" {
		return fmt.Errorf("cluster DNS address not discovered yet: refusing to publish bashible context without it")
	}
	input, err := s.Build(ctx, globals, nodeGroups, providers)
	if err != nil {
		return err
	}
	raw, err := Marshal(input)
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
