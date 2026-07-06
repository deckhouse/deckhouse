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
	"encoding/base64"
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	sigsyaml "sigs.k8s.io/yaml"
)

const (
	// versionInfoCM carries deckhouse channel/version/edition — the values the
	// helm path renders from deckhouse-image files (/deckhouse/version,
	// /deckhouse/edition) and the deckhouse ModuleConfig releaseChannel. The
	// 002-deckhouse module already materialises them into this ConfigMap, so the
	// node-controller (a different image) reads them from kube instead of files.
	versionInfoCMName = "d8-deckhouse-version-info"
	versionInfoCMNS   = "d8-system"

	// clusterConfigSecret holds the cluster-configuration.yaml the helm path
	// reads global.clusterConfiguration.* from (podSubnetNodeCIDRPrefix,
	// clusterDomain, podSubnetCIDR, serviceSubnetCIDR, proxy).
	clusterConfigSecretName = "d8-cluster-configuration"
	clusterConfigKey        = "cluster-configuration.yaml"

	// clusterUUIDConfigMap holds the cluster UUID (global.discovery.clusterUUID).
	clusterUUIDConfigMapName = "d8-cluster-uuid"
	clusterUUIDKey           = "cluster-uuid"

	dnsAppLabel = "k8s-app"
)

// ReadGlobals assembles the Globals block from live kube objects, mirroring the
// helm values the define bashible_input_data reads from. It replaces the earlier
// env-injection: every global field is now sourced from an already-materialised
// object (ConfigMap/Secret/Service), consistent with the rest of the package.
func (s *Service) ReadGlobals(ctx context.Context) Globals {
	g := Globals{}
	g.DeckhouseChannel, g.DeckhouseVersion, g.DeckhouseEdition = s.readDeckhouseInfo(ctx)
	g.ClusterUUID = s.readClusterUUID(ctx)
	g.ClusterDNSAddress = s.readClusterDNSAddress(ctx)
	if cfg := s.readClusterConfiguration(ctx); cfg != nil {
		g.PodSubnetNodeCIDRPrefix = cfg.PodSubnetNodeCIDRPrefix
		g.ClusterDomain = cfg.ClusterDomain
		g.Proxy = buildProxy(cfg)
	}
	return g
}

type deckhouseVersionInfo struct {
	Channel string `json:"channel"`
	Version string `json:"version"`
	Edition string `json:"edition"`
}

// readDeckhouseInfo reads the d8-system/d8-deckhouse-version-info ConfigMap's
// data.json ({channel,version,edition}). Empty strings when absent — Build then
// applies the same channel default ("unknown") as the template.
func (s *Service) readDeckhouseInfo(ctx context.Context) (channel, version, edition string) {
	cm := &corev1.ConfigMap{}
	if err := s.Client.Get(ctx, types.NamespacedName{Namespace: versionInfoCMNS, Name: versionInfoCMName}, cm); err != nil {
		return "", "", ""
	}
	var info deckhouseVersionInfo
	if err := json.Unmarshal([]byte(cm.Data["data.json"]), &info); err != nil {
		return "", "", ""
	}
	return info.Channel, info.Version, info.Edition
}

// readClusterUUID reads the kube-system/d8-cluster-uuid ConfigMap's cluster-uuid
// key ("" when absent — Build applies the all-zeros default like the template).
func (s *Service) readClusterUUID(ctx context.Context) string {
	cm := &corev1.ConfigMap{}
	if err := s.Client.Get(ctx, types.NamespacedName{Namespace: kubeSystemNS, Name: clusterUUIDConfigMapName}, cm); err != nil {
		return ""
	}
	return cm.Data[clusterUUIDKey]
}

type bashibleClusterConfiguration struct {
	PodSubnetNodeCIDRPrefix string                 `json:"podSubnetNodeCIDRPrefix"`
	ClusterDomain           string                 `json:"clusterDomain"`
	PodSubnetCIDR           string                 `json:"podSubnetCIDR"`
	ServiceSubnetCIDR       string                 `json:"serviceSubnetCIDR"`
	Proxy                   map[string]interface{} `json:"proxy,omitempty"`
}

// readClusterConfiguration parses the cluster-configuration.yaml Secret value,
// tolerating the extra base64 layer the way derived_status does. nil when the
// Secret or key is absent so the caller leaves the derived fields empty.
func (s *Service) readClusterConfiguration(ctx context.Context) *bashibleClusterConfiguration {
	secret := &corev1.Secret{}
	if err := s.Client.Get(ctx, types.NamespacedName{Namespace: kubeSystemNS, Name: clusterConfigSecretName}, secret); err != nil {
		return nil
	}
	raw, ok := secret.Data[clusterConfigKey]
	if !ok {
		return nil
	}
	if decoded, err := base64.StdEncoding.DecodeString(string(raw)); err == nil {
		raw = decoded
	}
	cfg := &bashibleClusterConfiguration{}
	if err := sigsyaml.Unmarshal(raw, cfg); err != nil {
		return nil
	}
	return cfg
}

// buildProxy mirrors the define's proxy block: httpProxy/httpsProxy passed
// through only when present, and noProxy = the fixed prefix
// (127.0.0.1, 169.254.169.254, clusterDomain, podSubnetCIDR, serviceSubnetCIDR)
// concatenated with any custom noProxy. nil when the config has no proxy key, so
// the whole block is omitted like `if hasKey clusterConfiguration "proxy"`.
func buildProxy(cfg *bashibleClusterConfiguration) map[string]interface{} {
	if cfg.Proxy == nil {
		return nil
	}
	proxy := map[string]interface{}{}
	if v, ok := cfg.Proxy["httpProxy"]; ok {
		proxy["httpProxy"] = v
	}
	if v, ok := cfg.Proxy["httpsProxy"]; ok {
		proxy["httpsProxy"] = v
	}
	noProxy := []interface{}{"127.0.0.1", "169.254.169.254", cfg.ClusterDomain, cfg.PodSubnetCIDR, cfg.ServiceSubnetCIDR}
	if custom, ok := cfg.Proxy["noProxy"].([]interface{}); ok {
		noProxy = append(noProxy, custom...)
	}
	proxy["noProxy"] = noProxy
	return proxy
}

// readClusterDNSAddress mirrors discovery/cluster_dns_address: the ClusterIP of
// the kube-system Service labelled k8s-app in (kube-dns, coredns), preferring the
// one named "kube-dns", skipping headless ("None"/"") services.
func (s *Service) readClusterDNSAddress(ctx context.Context) string {
	list := &corev1.ServiceList{}
	if err := s.Client.List(ctx, list, client.InNamespace(kubeSystemNS)); err != nil {
		return ""
	}
	dnsAddress := ""
	for i := range list.Items {
		svc := &list.Items[i]
		app := svc.Labels[dnsAppLabel]
		if app != "kube-dns" && app != "coredns" {
			continue
		}
		ip := svc.Spec.ClusterIP
		if ip == "None" || ip == "" {
			continue
		}
		if svc.Name == "kube-dns" {
			return ip
		}
		dnsAddress = ip
	}
	return dnsAddress
}
