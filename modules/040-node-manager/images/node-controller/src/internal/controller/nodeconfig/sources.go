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

package nodeconfig

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	sigsyaml "sigs.k8s.io/yaml"
)

// clusterInputs is everything outside the NodeGroup that a rendered NodeConfig
// needs. It is read once per reconcile pass from the cluster itself: the
// controller has no access to Deckhouse values.
type clusterInputs struct {
	// APIServerEndpoints are the addresses the node-local proxy balances over.
	APIServerEndpoints []string
	// ClusterDomain and ClusterDNS configure kubelet's DNS.
	ClusterDomain string
	ClusterDNS    string
	// KubernetesCA is the base64-encoded cluster CA. kubelet loads it from disk
	// on every start, and on an immutable node that file lives on tmpfs — so a
	// config without the CA leaves the node unable to start kubelet after a
	// reboot, with no way to get the certificate back.
	KubernetesCA string
	// SysextDigests maps an extension name to the image digest to pull.
	SysextDigests map[string]string
	// RegistryPackagesProxyToken authenticates against the packages proxy.
	RegistryPackagesProxyToken string
}

// sourceReader reads cluster state. It falls back to the cached client when no
// uncached reader was injected.
type sourceReader struct {
	Client client.Client
	Reader client.Reader
}

func (s *sourceReader) reader() client.Reader {
	if s.Reader != nil {
		return s.Reader
	}
	return s.Client
}

// readClusterInputs collects everything a NodeConfig is rendered from. Missing
// pieces are reported: rendering a config with, say, no API server endpoints
// would strand the node.
func (s *sourceReader) readClusterInputs(ctx context.Context, kubernetesVersion string) (clusterInputs, error) {
	in := clusterInputs{}

	in.APIServerEndpoints = s.readAPIServerEndpoints(ctx)
	if len(in.APIServerEndpoints) == 0 {
		return in, fmt.Errorf("no API server endpoints discovered")
	}

	in.ClusterDomain, in.ClusterDNS = s.readDNS(ctx)

	ca, err := s.readClusterCA(ctx)
	if err != nil {
		return in, err
	}
	in.KubernetesCA = ca

	digests, err := s.readSysextDigests(ctx, kubernetesVersion)
	if err != nil {
		return in, err
	}
	in.SysextDigests = digests

	token, err := s.readPackagesProxyToken(ctx)
	if err != nil {
		return in, err
	}
	in.RegistryPackagesProxyToken = token

	return in, nil
}

// readClusterCA returns the cluster CA, base64-encoded the way the NodeConfig
// carries it. It comes from the ConfigMap Kubernetes publishes for every
// ServiceAccount, so there is nothing module-specific to keep in sync.
func (s *sourceReader) readClusterCA(ctx context.Context) (string, error) {
	cm := &corev1.ConfigMap{}
	if err := s.reader().Get(ctx, types.NamespacedName{Namespace: kubeSystemNS, Name: clusterCAConfigMap}, cm); err != nil {
		return "", fmt.Errorf("read the cluster CA from %s/%s: %w", kubeSystemNS, clusterCAConfigMap, err)
	}
	ca := cm.Data[clusterCAKey]
	if ca == "" {
		return "", fmt.Errorf("configmap %s/%s carries no %s", kubeSystemNS, clusterCAConfigMap, clusterCAKey)
	}
	return base64.StdEncoding.EncodeToString([]byte(ca)), nil
}

// readAPIServerEndpoints merges the control-plane pod IPs with the kubernetes
// EndpointSlice, the same two sources bashible is given.
func (s *sourceReader) readAPIServerEndpoints(ctx context.Context) []string {
	set := make(map[string]struct{})

	pods := &corev1.PodList{}
	if err := s.reader().List(ctx, pods,
		client.InNamespace(kubeSystemNS),
		client.MatchingLabels{"component": "kube-apiserver", "tier": "control-plane"},
	); err == nil {
		for i := range pods.Items {
			pod := &pods.Items[i]
			if !podReady(pod) || pod.Status.PodIP == "" {
				continue
			}
			set[net.JoinHostPort(pod.Status.PodIP, strconv.Itoa(apiserverPort))] = struct{}{}
		}
	}

	slice := &discoveryv1.EndpointSlice{}
	if err := s.reader().Get(ctx, types.NamespacedName{Namespace: "default", Name: "kubernetes"}, slice); err == nil {
		var ports []int32
		for _, port := range slice.Ports {
			if port.Name != nil && *port.Name == "https" && port.Port != nil {
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
	}

	list := make([]string, 0, len(set))
	for ep := range set {
		if ep == "" {
			continue
		}
		list = append(list, "https://"+ep)
	}
	sort.Strings(list)
	return list
}

func podReady(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

// readDNS returns the cluster domain and the in-cluster DNS service address.
func (s *sourceReader) readDNS(ctx context.Context) (string, string) {
	domain := "cluster.local"
	dns := ""

	secret := &corev1.Secret{}
	if err := s.reader().Get(ctx, types.NamespacedName{Namespace: kubeSystemNS, Name: clusterConfigSecretName}, secret); err == nil {
		if raw, ok := secret.Data[clusterConfigKey]; ok {
			var cfg struct {
				ClusterDomain string `json:"clusterDomain"`
			}
			if err := sigsyaml.Unmarshal(raw, &cfg); err == nil && cfg.ClusterDomain != "" {
				domain = cfg.ClusterDomain
			}
		}
	}

	list := &corev1.ServiceList{}
	if err := s.reader().List(ctx, list, client.InNamespace(kubeSystemNS)); err == nil {
		for i := range list.Items {
			svc := &list.Items[i]
			app := svc.Labels[dnsAppLabel]
			if app != "kube-dns" && app != "coredns" {
				continue
			}
			ip := svc.Spec.ClusterIP
			if ip == "" || ip == corev1.ClusterIPNone {
				continue
			}
			if svc.Name == "kube-dns" {
				return domain, ip
			}
			dns = ip
		}
	}
	return domain, dns
}

// readSysextDigests picks the system extension digests for this release: one
// containerd, one CNI, and the kubelet matching the group's Kubernetes version.
// The digests live in the same ConfigMap bashible-apiserver reads.
func (s *sourceReader) readSysextDigests(ctx context.Context, kubernetesVersion string) (map[string]string, error) {
	cm := &corev1.ConfigMap{}
	if err := s.reader().Get(ctx, types.NamespacedName{Namespace: cloudInstanceManagerNS, Name: imagesDigestsConfigMapName}, cm); err != nil {
		return nil, fmt.Errorf("read image digests: %w", err)
	}

	raw, ok := cm.Data[imagesDigestsKey]
	if !ok {
		return nil, fmt.Errorf("configmap %s/%s has no %q key", cloudInstanceManagerNS, imagesDigestsConfigMapName, imagesDigestsKey)
	}

	var all map[string]map[string]string
	if err := json.Unmarshal([]byte(raw), &all); err != nil {
		return nil, fmt.Errorf("parse image digests: %w", err)
	}

	packages := all[registryPackagesDigestsKey]
	if len(packages) == 0 {
		return nil, fmt.Errorf("no %q digests in %s", registryPackagesDigestsKey, imagesDigestsKey)
	}

	digests := make(map[string]string, 3)

	// The image names carry the version with the separators stripped:
	// containerdSysext224, kubernetesCniSysext162, kubeletSysext1356.
	if d := pickDigest(packages, "containerdSysext"); d != "" {
		digests[containerdExtension] = d
	}
	if d := pickDigest(packages, "kubernetesCniSysext"); d != "" {
		digests[cniExtension] = d
	}
	if d := pickKubeletDigest(packages, kubernetesVersion); d != "" {
		digests[kubeletExtension] = d
	}

	for _, name := range []string{containerdExtension, cniExtension, kubeletExtension} {
		if digests[name] == "" {
			return nil, fmt.Errorf("no %s system extension digest for Kubernetes %s", name, kubernetesVersion)
		}
	}
	return digests, nil
}

// pickDigest returns the digest of the newest image with the given prefix.
// Newest is the highest version suffix, compared as a plain string because the
// suffixes are zero-free digit runs of the same shape.
func pickDigest(packages map[string]string, prefix string) string {
	best, bestKey := "", ""
	for name, digest := range packages {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		if name > bestKey {
			best, bestKey = digest, name
		}
	}
	return best
}

// pickKubeletDigest finds the kubelet extension for a Kubernetes minor version:
// kubeletSysext1356 serves 1.35. Falls back to the newest build of that minor.
func pickKubeletDigest(packages map[string]string, kubernetesVersion string) string {
	minor := strings.ReplaceAll(kubernetesVersion, ".", "")
	if minor == "" {
		return ""
	}
	return pickDigest(packages, "kubeletSysext"+minor)
}

// readPackagesProxyToken returns the token the node presents to the registry
// packages proxy, base64-encoded the way the on-node agent expects it.
func (s *sourceReader) readPackagesProxyToken(ctx context.Context) (string, error) {
	secret := &corev1.Secret{}
	if err := s.reader().Get(ctx, types.NamespacedName{Namespace: cloudInstanceManagerNS, Name: registryPackagesProxyTokenSecret}, secret); err != nil {
		return "", fmt.Errorf("read registry packages proxy token: %w", err)
	}
	token, ok := secret.Data[registryPackagesProxyTokenKey]
	if !ok || len(token) == 0 {
		return "", fmt.Errorf("secret %s/%s has no %q key", cloudInstanceManagerNS, registryPackagesProxyTokenSecret, registryPackagesProxyTokenKey)
	}
	// The agent decodes the field before use, and the secret already holds the
	// raw token, so hand it over encoded.
	return base64.StdEncoding.EncodeToString(token), nil
}
