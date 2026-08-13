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
	"errors"
	"fmt"
	"maps"
	"net"
	"slices"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	sigsyaml "sigs.k8s.io/yaml"

	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// clusterInputs is everything outside the NodeGroup that a rendered NodeConfig
// needs. It is read once per reconcile pass from the cluster itself: the
// controller has no access to Deckhouse values.
type clusterInputs struct {
	// APIServerEndpoints are the addresses the node-local proxy balances over.
	APIServerEndpoints []string
	// KubernetesVersion is the cluster's minor version, e.g. "1.34". kubelet's
	// feature gates depend on it (bashible turns DRA gates on by version), so a
	// node not told the version cannot follow.
	KubernetesVersion string
	// OSImage is the olcedar image of this release, pinned by digest. The node
	// compares it with the digest it recorded at install and updates its root
	// when they differ, so a tag here would make every update undecidable.
	OSImage internalv1alpha1.OSImage
	// ClusterDomain and ClusterDNS configure kubelet's DNS.
	ClusterDomain string
	ClusterDNS    string
	// DefaultMaxPods is the pods-per-node ceiling for a group that does not set
	// one, derived from the pod subnet the way bashible derives it. A node that
	// disagrees with the rest of the fleet skews the scheduler for all of it.
	DefaultMaxPods int
	// KubernetesCA is the base64-encoded cluster CA. kubelet loads it from disk
	// on every start, and on an immutable node that file lives on tmpfs — a
	// config without it leaves the node unable to start kubelet after a reboot.
	KubernetesCA string
	// SysextDigests maps an extension name to the image digest to pull.
	SysextDigests map[string]string
	// RegistryPackagesProxyToken authenticates against the packages proxy.
	RegistryPackagesProxyToken string
	// SandboxImage is the pause image, resolved against the cluster's own
	// registry: a cluster installed from a private registry has no route to the
	// upstream one, and a node that cannot pull pause runs no pods at all.
	SandboxImage string
	// Registry is how a node reaches the cluster's registry on its own. Every
	// node gets it: containerd pulls pause with no imagePullSecret, so a worker
	// without credentials fails every sandbox it tries to create.
	Registry *internalv1alpha1.Registry
	// NodeExtensions are the operator's requests to merge extra system extensions
	// onto the nodes they select, in the order their uniqueness contest ran; the
	// contest is cluster-wide, so it is settled here, once per pass, rather than
	// once per node. NodeExtensionConflicts says which requests lost it.
	NodeExtensions         []*deckhousev1alpha1.NodeExtensionRequest
	NodeExtensionConflicts map[string]nerConflict
}

// sourceReader reads cluster state. Reader goes straight to the API server
// (decisions followed by writes; the cache is a beat behind); Client is cached,
// used only for kinds this controller watches. Deliberately no fallback.
type sourceReader struct {
	Client client.Client
	Reader client.Reader
}

// readClusterInputs collects everything a NodeConfig is rendered from. Every
// read is fail-closed: falling back on a default would roll a wrong value out
// node by node. The cost: while any read fails, no immutable node is rendered.
func (s *sourceReader) readClusterInputs(ctx context.Context, kubernetesVersion string) (clusterInputs, error) {
	in := clusterInputs{KubernetesVersion: kubernetesVersion}

	endpoints, err := s.readAPIServerEndpoints(ctx)
	if err != nil {
		return in, err
	}
	if len(endpoints) == 0 {
		return in, errors.New("no API server endpoints discovered")
	}
	in.APIServerEndpoints = endpoints

	config, err := s.readClusterConfiguration(ctx)
	if err != nil {
		return in, err
	}
	in.ClusterDomain = config.clusterDomain()
	in.DefaultMaxPods = defaultMaxPodsFor(config.PodSubnetNodeCIDRPrefix)

	dns, err := s.readClusterDNS(ctx)
	if err != nil {
		return in, err
	}
	in.ClusterDNS = dns

	ca, err := s.readClusterCA(ctx)
	if err != nil {
		return in, err
	}
	in.KubernetesCA = ca

	// One read for both the system extensions and the pause image: they come out
	// of the same ConfigMap, and a render pass that reads it twice pays twice.
	images, err := s.readImagesDigests(ctx)
	if err != nil {
		return in, err
	}

	digests, err := sysextDigests(images, kubernetesVersion)
	if err != nil {
		return in, err
	}
	in.SysextDigests = digests

	token, err := s.readPackagesProxyToken(ctx)
	if err != nil {
		return in, err
	}
	in.RegistryPackagesProxyToken = token

	registry, imagesRepo, err := s.readRegistry(ctx)
	if err != nil {
		return in, err
	}
	in.Registry = registry

	osImage, err := osImageDigest(images)
	if err != nil {
		return in, err
	}
	in.OSImage = internalv1alpha1.OSImage{Digest: osImage}

	sandbox, err := sandboxImage(images, imagesRepo)
	if err != nil {
		return in, err
	}
	in.SandboxImage = sandbox

	ners, err := s.readNodeExtensionRequests(ctx)
	if err != nil {
		return in, err
	}
	in.NodeExtensions = orderedNERs(ners)
	in.NodeExtensionConflicts = resolveNERConflicts(in.NodeExtensions)

	return in, nil
}

// reportedNodeIPs returns the node's reported internal addresses, read from the
// API server (the manager's cache strips Node.status.addresses). Nothing is
// read unless the config pins an address — only the bootstrapped first master.
func (r *Reconciler) reportedNodeIPs(ctx context.Context, nodeName, pinned string) ([]string, error) {
	if pinned == "" {
		return nil, nil
	}
	node := &corev1.Node{}
	if err := r.sources.Reader.Get(ctx, types.NamespacedName{Name: nodeName}, node); err != nil {
		return nil, fmt.Errorf("read the addresses of node %s: %w", nodeName, err)
	}
	var addresses []string
	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeInternalIP {
			addresses = append(addresses, address.Address)
		}
	}
	return addresses, nil
}

// readRegistry describes the cluster's registry: the spec a node needs to reach
// it, and the repository every image of the release lives in.
func (s *sourceReader) readRegistry(ctx context.Context) (*internalv1alpha1.Registry, string, error) {
	secret := &corev1.Secret{}
	if err := s.Reader.Get(ctx, types.NamespacedName{Namespace: d8SystemNS, Name: deckhouseRegistrySecret}, secret); err != nil {
		return nil, "", fmt.Errorf("read the registry configuration from %s/%s: %w", d8SystemNS, deckhouseRegistrySecret, err)
	}

	address := string(secret.Data[registryAddressKey])
	if address == "" {
		return nil, "", fmt.Errorf("secret %s/%s carries no %q", d8SystemNS, deckhouseRegistrySecret, registryAddressKey)
	}

	auth, err := registryAuth(secret.Data[registryDockerConfigKey], address)
	if err != nil {
		return nil, "", fmt.Errorf("read the registry credentials from %s/%s: %w", d8SystemNS, deckhouseRegistrySecret, err)
	}

	registry := &internalv1alpha1.Registry{
		Address: address,
		Path:    string(secret.Data[registryPathKey]),
		Scheme:  strings.ToUpper(string(secret.Data[registrySchemeKey])),
		CA:      string(secret.Data[registryCAKey]),
		Auth:    auth,
	}

	imagesRepo := string(secret.Data[registryImagesKey])
	if imagesRepo == "" {
		imagesRepo = address + registry.Path
	}
	return registry, imagesRepo, nil
}

// registryAuth pulls one registry's credentials out of a docker config. No
// credentials is fine (anonymous pull), but an unparsable config is an error —
// treating it as anonymous fails against a private registry with no cause shown.
func registryAuth(dockerConfig []byte, address string) (string, error) {
	if len(dockerConfig) == 0 {
		return "", nil
	}
	var config struct {
		Auths map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}
	if err := json.Unmarshal(dockerConfig, &config); err != nil {
		return "", fmt.Errorf("parse %s: %w", registryDockerConfigKey, err)
	}
	return config.Auths[address].Auth, nil
}

// sandboxImage resolves the pause image against the cluster's own registry.
func sandboxImage(images map[string]map[string]string, imagesRepo string) (string, error) {
	digest := images[pauseDigestGroup][pauseDigestName]
	if digest == "" {
		return "", fmt.Errorf("no %s/%s digest in %s", pauseDigestGroup, pauseDigestName, imagesDigestsKey)
	}
	return imagesRepo + "@" + digest, nil
}

// readNodeExtensionRequests lists the extension requests; an empty list is fine.
// The one cached read here: the controller watches this kind, and a live read
// could disagree with the status pass, which reports from the cached list.
func (s *sourceReader) readNodeExtensionRequests(ctx context.Context) ([]deckhousev1alpha1.NodeExtensionRequest, error) {
	list := &deckhousev1alpha1.NodeExtensionRequestList{}
	if err := s.Client.List(ctx, list); err != nil {
		return nil, fmt.Errorf("list node extension requests: %w", err)
	}
	return list.Items, nil
}

// readClusterCA returns the cluster CA, base64-encoded the way the NodeConfig
// carries it. It comes from the ConfigMap Kubernetes publishes for every
// ServiceAccount, so there is nothing module-specific to keep in sync.
func (s *sourceReader) readClusterCA(ctx context.Context) (string, error) {
	cm := &corev1.ConfigMap{}
	if err := s.Reader.Get(ctx, types.NamespacedName{Namespace: kubeSystemNS, Name: clusterCAConfigMap}, cm); err != nil {
		return "", fmt.Errorf("read the cluster CA from %s/%s: %w", kubeSystemNS, clusterCAConfigMap, err)
	}
	ca := cm.Data[clusterCAKey]
	if ca == "" {
		return "", fmt.Errorf("configmap %s/%s carries no %s", kubeSystemNS, clusterCAConfigMap, clusterCAKey)
	}
	return base64.StdEncoding.EncodeToString([]byte(ca)), nil
}

// readAPIServerEndpoints merges control-plane pod IPs with the kubernetes
// EndpointSlice. Deliberately readiness-blind: these land in every node's spec,
// and restart churn would re-render the fleet. Terminating pods are dropped.
func (s *sourceReader) readAPIServerEndpoints(ctx context.Context) ([]string, error) {
	set := make(map[string]struct{})

	pods := &corev1.PodList{}
	if err := s.Reader.List(ctx, pods,
		client.InNamespace(kubeSystemNS),
		client.MatchingLabels{"component": "kube-apiserver", "tier": "control-plane"},
	); err != nil {
		return nil, fmt.Errorf("list kube-apiserver pods: %w", err)
	}
	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.DeletionTimestamp != nil || pod.Status.PodIP == "" {
			continue
		}
		set["https://"+net.JoinHostPort(pod.Status.PodIP, strconv.Itoa(apiserverPort))] = struct{}{}
	}

	slice := &discoveryv1.EndpointSlice{}
	if err := s.Reader.Get(ctx, types.NamespacedName{Namespace: apiServerEndpointSliceNS, Name: apiServerEndpointSliceName}, slice); err != nil {
		return nil, fmt.Errorf("read the %s/%s EndpointSlice: %w", apiServerEndpointSliceNS, apiServerEndpointSliceName, err)
	}
	var ports []int32
	for _, port := range slice.Ports {
		if port.Name == nil || *port.Name != apiServerPortName {
			continue
		}
		if port.Port == nil {
			continue
		}
		ports = append(ports, *port.Port)
	}
	for _, endpoint := range slice.Endpoints {
		for _, addr := range endpoint.Addresses {
			for _, port := range ports {
				set["https://"+net.JoinHostPort(addr, strconv.Itoa(int(port)))] = struct{}{}
			}
		}
	}

	return slices.Sorted(maps.Keys(set)), nil
}

// readClusterDNS returns the in-cluster DNS service address. Finding none is an
// error, not an empty answer: a pass that saw no DNS service would publish a
// DNS-less config to every immutable node and roll it out as a change.
func (s *sourceReader) readClusterDNS(ctx context.Context) (string, error) {
	list := &corev1.ServiceList{}
	if err := s.Reader.List(ctx, list, client.InNamespace(kubeSystemNS)); err != nil {
		return "", fmt.Errorf("list the DNS services in %s: %w", kubeSystemNS, err)
	}

	dns := ""
	chosen := ""
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
		if svc.Name == kubeDNSServiceName {
			return ip, nil
		}
		// Whichever candidate the list happened to end on used to win, so two
		// DNS services meant a cluster DNS address that changed on its own. The
		// first name in order is an arbitrary choice, but the same one every pass.
		if chosen == "" || svc.Name < chosen {
			chosen, dns = svc.Name, ip
		}
	}
	if dns == "" {
		return "", fmt.Errorf("no DNS service with %s in (kube-dns, coredns) and a cluster IP in %s", dnsAppLabel, kubeSystemNS)
	}
	return dns, nil
}

// clusterConfiguration is the part of the cluster's own configuration a node
// config is rendered from.
type clusterConfiguration struct {
	ClusterDomain string `json:"clusterDomain"`
	// PodSubnetNodeCIDRPrefix is how much of the pod subnet each node gets, and
	// so how many pods can fit on one. ClusterConfiguration writes it as a
	// string, and dhctl's rendered copy as a number, so it is read as either.
	PodSubnetNodeCIDRPrefix intstr.IntOrString `json:"podSubnetNodeCIDRPrefix"`
}

// clusterDomain is the configured domain, or the default ClusterConfiguration
// applies when the field is left out.
func (c clusterConfiguration) clusterDomain() string {
	if c.ClusterDomain == "" {
		return defaultClusterDomain
	}
	return c.ClusterDomain
}

// readClusterConfiguration reads the cluster configuration secret. A failure is
// reported rather than replaced by defaults: quietly rendering defaults would
// reconfigure every node of every immutable group.
func (s *sourceReader) readClusterConfiguration(ctx context.Context) (clusterConfiguration, error) {
	secret := &corev1.Secret{}
	if err := s.Reader.Get(ctx, types.NamespacedName{Namespace: kubeSystemNS, Name: clusterConfigSecretName}, secret); err != nil {
		return clusterConfiguration{}, fmt.Errorf("read the cluster configuration from %s/%s: %w", kubeSystemNS, clusterConfigSecretName, err)
	}
	raw, ok := secret.Data[clusterConfigKey]
	if !ok {
		return clusterConfiguration{}, fmt.Errorf("secret %s/%s carries no %q", kubeSystemNS, clusterConfigSecretName, clusterConfigKey)
	}
	var config clusterConfiguration
	if err := sigsyaml.Unmarshal(raw, &config); err != nil {
		return clusterConfiguration{}, fmt.Errorf("parse %s/%s: %w", kubeSystemNS, clusterConfigSecretName, err)
	}
	return config, nil
}

// defaultMaxPodsFor derives the default pods-per-node from the pod subnet, as
// bashible does (candi/bashible/common-steps/all/064_configure_kubelet.sh.tpl) —
// no scheduler skew between node kinds; dhctl mirrors it (pkg/immutable/nodeconfig.go).
func defaultMaxPodsFor(prefix intstr.IntOrString) int {
	byPrefix := map[int]int{24: 120, 23: 250, 22: 500, 21: 1000}
	bits := prefix.IntValue()
	if bits == 0 {
		bits = defaultPodSubnetNodeCIDRPrefix
	}
	// A prefix outside the ladder takes the nearest step, as the template does.
	return byPrefix[min(max(bits, 21), 24)]
}

// readImagesDigests returns the digest of every image the release ships, keyed
// by group and then by image name. The group is a key in this map, not a path
// segment: every Deckhouse image lives in one repository, addressed by digest.
func (s *sourceReader) readImagesDigests(ctx context.Context) (map[string]map[string]string, error) {
	cm := &corev1.ConfigMap{}
	if err := s.Reader.Get(ctx, types.NamespacedName{Namespace: cloudInstanceManagerNS, Name: imagesDigestsConfigMapName}, cm); err != nil {
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
	return all, nil
}

// sysextDigests picks the system extension digests for this release: one
// containerd, one CNI, and the kubelet matching the group's Kubernetes version.
// The digests live in the same ConfigMap bashible-apiserver reads.
func sysextDigests(all map[string]map[string]string, kubernetesVersion string) (map[string]string, error) {
	packages := all[registryPackagesDigestsKey]
	if len(packages) == 0 {
		return nil, fmt.Errorf("no %q digests in %s", registryPackagesDigestsKey, imagesDigestsKey)
	}

	digests := make(map[string]string, 4)

	// The image names carry the version with the separators stripped:
	// containerdSysext224, kubernetesCniSysext162, kubeletSysext1356.
	containerd, err := soleDigest(packages, "containerdSysext")
	if err != nil {
		return nil, err
	}
	digests[containerdExtension] = containerd

	cni, err := soleDigest(packages, "kubernetesCniSysext")
	if err != nil {
		return nil, err
	}
	digests[cniExtension] = cni

	// Read by exact key: the agent image has no version in its name, so the
	// numeric tail soleDigest looks for is absent by construction.
	nodelet := packages[nodeletSysextImage]
	if nodelet == "" {
		return nil, fmt.Errorf("no %s system extension digest in %s", nodeletExtension, imagesDigestsKey)
	}
	digests[nodeletExtension] = nodelet

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

// osImageDigest picks the olcedar image of this release. Absent means the
// release did not build it, and rendering stops: a node told no OS image can
// neither be installed nor updated, and guessing one would point it at whatever
// the previous release shipped.
func osImageDigest(all map[string]map[string]string) (string, error) {
	digest := all[nodeManagerDigestsKey][osImageName]
	if digest == "" {
		return "", fmt.Errorf("no %q OS image digest in %s", osImageName, imagesDigestsKey)
	}
	return digest, nil
}

// versionedImages returns the images named the prefix followed by a version, as
// image name to version. Everything after the prefix is the version, so a
// non-numeric tail is another image whose name merely starts the same way.
func versionedImages(packages map[string]string, prefix string) map[string]int {
	found := map[string]int{}
	for name := range packages {
		suffix, ok := strings.CutPrefix(name, prefix)
		if !ok {
			continue
		}
		version, err := strconv.Atoi(suffix)
		if err != nil {
			continue
		}
		found[name] = version
	}
	return found
}

// soleDigest returns the digest of the one image with the given prefix. It picks
// no newest because none can be told: "kubernetesCniSysext1610" is ambiguous.
// Several is a build defect, reported. Kept in step with dhctl's soleDigest.
func soleDigest(packages map[string]string, prefix string) (string, error) {
	found := slices.Sorted(maps.Keys(versionedImages(packages, prefix)))
	switch len(found) {
	case 0:
		return "", nil
	case 1:
		return packages[found[0]], nil
	default:
		return "", fmt.Errorf("the release carries %d %q system extensions (%s): their names do not say which one is newer",
			len(found), prefix, strings.Join(found, ", "))
	}
}

// newestPatchDigest returns the newest image with the given prefix, which pins
// everything but the patch, so the suffix compares numerically — a string
// compare would put patch 6 over patch 10.
func newestPatchDigest(packages map[string]string, prefix string) string {
	best, bestVersion := "", -1
	for name, version := range versionedImages(packages, prefix) {
		if version > bestVersion {
			best, bestVersion = packages[name], version
		}
	}
	return best
}

// pickKubeletDigest returns the newest patch of the kubelet extension serving a
// Kubernetes minor version: for 1.35 the prefix pins kubeletSysext135, and the
// remaining suffix is the patch alone.
func pickKubeletDigest(packages map[string]string, kubernetesVersion string) string {
	minor := strings.ReplaceAll(kubernetesVersion, ".", "")
	if minor == "" {
		return ""
	}
	return newestPatchDigest(packages, "kubeletSysext"+minor)
}

// readPackagesProxyToken returns the token the node presents to the registry
// packages proxy, base64-encoded the way the on-node agent expects it.
func (s *sourceReader) readPackagesProxyToken(ctx context.Context) (string, error) {
	secret := &corev1.Secret{}
	if err := s.Reader.Get(ctx, types.NamespacedName{Namespace: cloudInstanceManagerNS, Name: registryPackagesProxyTokenSecret}, secret); err != nil {
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
