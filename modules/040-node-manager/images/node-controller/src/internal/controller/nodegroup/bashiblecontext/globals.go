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
	versionInfoCMName = "d8-deckhouse-version-info"
	versionInfoCMNS   = "d8-system"

	clusterConfigSecretName = "d8-cluster-configuration"
	clusterConfigKey        = "cluster-configuration.yaml"

	// clusterUUIDConfigMap holds the cluster UUID (global.discovery.clusterUUID).
	clusterUUIDConfigMapName = "d8-cluster-uuid"
	clusterUUIDKey           = "cluster-uuid"

	dnsAppLabel = "k8s-app"
)

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

func (s *Service) readDeckhouseInfo(ctx context.Context) (string, string, string) {
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

func (s *Service) readClusterConfiguration(ctx context.Context) *bashibleClusterConfiguration {
	secret := &corev1.Secret{}
	if err := s.reader().Get(ctx, types.NamespacedName{Namespace: kubeSystemNS, Name: clusterConfigSecretName}, secret); err != nil {
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
