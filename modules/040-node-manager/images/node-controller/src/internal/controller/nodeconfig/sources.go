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
	"net"
	"sort"
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
	// feature gates depend on it: bashible turns the DRA gates on by version, and
	// a node that is not told the version cannot follow — DRA would then work on
	// every node in the cluster except the immutable ones.
	KubernetesVersion string
	// OSImage is the olcedar image, addressed in the cluster's own registry. It
	// is a record rather than an instruction (the node boots whatever its
	// InstanceClass points the root disk at, see NodeSpec.OSImage), but it still
	// has to name the image the installer named: a record that disagrees with
	// what the node was installed from is a record of nothing, and naming the
	// public registry tells an air-gapped operator to look somewhere unreachable.
	OSImage string
	// ClusterDomain and ClusterDNS configure kubelet's DNS.
	ClusterDomain string
	ClusterDNS    string
	// DefaultMaxPods is the pods-per-node ceiling for a group that does not set
	// one, derived from the pod subnet the way bashible derives it. A node that
	// disagrees with the rest of the fleet skews the scheduler for all of it.
	DefaultMaxPods int
	// KubernetesCA is the base64-encoded cluster CA. kubelet loads it from disk
	// on every start, and on an immutable node that file lives on tmpfs — so a
	// config without the CA leaves the node unable to start kubelet after a
	// reboot, with no way to get the certificate back.
	KubernetesCA string
	// SysextDigests maps an extension name to the image digest to pull.
	SysextDigests map[string]string
	// RegistryPackagesProxyToken authenticates against the packages proxy.
	RegistryPackagesProxyToken string
	// SandboxImage is the pause image containerd starts every pod sandbox from,
	// resolved against the cluster's own registry. It cannot be a constant: a
	// cluster installed from a private registry — the normal case in a closed
	// network — has no route to the upstream one, and a node that cannot pull
	// the pause image runs no pods at all.
	SandboxImage string
	// Registry is how a node reaches the cluster's registry on its own. Every
	// node gets it, not only the control-plane ones: containerd pulls the pause
	// image before any pod exists and with no imagePullSecret to use, so a worker
	// without credentials fails every sandbox it tries to create.
	Registry *internalv1alpha1.Registry
	// NodeExtensionRequests are the operator's requests to merge extra system
	// extensions onto the nodes they select.
	NodeExtensionRequests []deckhousev1alpha1.NodeExtensionRequest
}

// sourceReader reads cluster state. Both readers are required and mean
// different things: Reader goes straight to the API server and is what almost
// everything here uses, because these are decisions followed by a write and the
// cache is a beat behind; Client is the cached one, used only where this
// controller watches the kind itself.
//
// There is deliberately no fallback from one to the other. A reader that
// silently became the cached client is how a fix for a stale node address once
// shipped inert: it read a field the cache strips, passed its test against a
// hand-built object, and did nothing in production.
type sourceReader struct {
	Client client.Client
	Reader client.Reader
}

// readClusterInputs collects everything a NodeConfig is rendered from.
//
// Every read here is fail-closed, deliberately and more widely than it once was:
// an unreadable registry secret, an absent cluster configuration, a missing key
// in it, a body that will not parse, no DNS service — each aborts the render
// rather than falling back on a default. The alternative is not "nothing
// happens": the render would succeed carrying a plausible-looking value, the
// spec would differ from what the nodes run, and the group would roll that
// difference out node by node as if an operator had asked for it. A node
// reconfigured to no cluster DNS, or to "cluster.local" in a cluster that is
// not, is worse off than a node left running the config it already has while
// the cause is fixed.
//
// What that costs, stated rather than discovered: while any of these cannot be
// read, no immutable node is rendered — and RenderBootstrapSpec shares this
// function, so an immutable worker's bootstrap payload cannot be built either,
// which holds up new nodes as well as changes to existing ones. Nodes already
// running keep their config throughout; nothing is taken away from them.
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
	in.OSImage = registry.Address + registry.Path + "/" + osImageNameAndTag

	sandbox, err := sandboxImage(images, imagesRepo)
	if err != nil {
		return in, err
	}
	in.SandboxImage = sandbox

	ners, err := s.readNodeExtensionRequests(ctx)
	if err != nil {
		return in, err
	}
	in.NodeExtensionRequests = ners

	return in, nil
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

// registryAuth pulls the credentials for one registry out of a docker config.
// An anonymous registry has none, and that is not an error: the field is
// optional and a node without it pulls anonymously, exactly as the secret says.
// A docker config that cannot be parsed is an error, though — treating it as
// "no credentials" hands the node an anonymous pull that fails against a private
// registry with nothing saying why.
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

// readNodeExtensionRequests lists the extension requests. They are additive, so
// an empty list is fine: the node simply gets the base system extensions.
//
// This is the one read here served from the cache: the controller watches this
// kind, so the informer exists and is fed by the same events that trigger the
// pass. Reading it live would also disagree with the status pass, which reports
// each request's outcome from the cached list — a request could be rendered as a
// winner while its own status called it a loser.
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

// readAPIServerEndpoints merges the control-plane pod IPs with the kubernetes
// EndpointSlice, the two sources bashible discovers them from.
//
// Unlike bashible's, this list does not follow pod readiness, and deliberately:
// every address here lands in spec.apiServerEndpoints, so a set that shrinks
// when an apiserver is restarted changes the desired state of every node of
// every immutable group — a generation bump each, a rollout slot each, and a
// candidate interruption each, for a master that is coming back in seconds.
// Bashible pays nothing for the same churn: its endpoints live in a Secret the
// node re-reads, not in the node's own desired state. The node-local proxy
// balances away from an endpoint that does not answer, which is where an
// apiserver being momentarily down belongs.
//
// A master that is really gone still leaves, because the pod object goes with
// it. One that is on its way out is dropped as soon as it is asked to: a
// terminating mirror pod keeps its status.podIP to the end, so without this a
// drained, evicted or deleted master would hold its address in every node's
// config until something finally removed the object.
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
		set[net.JoinHostPort(pod.Status.PodIP, strconv.Itoa(apiserverPort))] = struct{}{}
	}

	slice := &discoveryv1.EndpointSlice{}
	if err := s.Reader.Get(ctx, types.NamespacedName{Namespace: apiServerEndpointSliceNS, Name: apiServerEndpointSliceName}, slice); err != nil {
		return nil, fmt.Errorf("read the %s/%s EndpointSlice: %w", apiServerEndpointSliceNS, apiServerEndpointSliceName, err)
	}
	var ports []int32
	for _, port := range slice.Ports {
		if port.Name != nil && *port.Name == apiServerPortName && port.Port != nil {
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

	list := make([]string, 0, len(set))
	for ep := range set {
		list = append(list, "https://"+ep)
	}
	sort.Strings(list)
	return list, nil
}

// readClusterDNS returns the address of the in-cluster DNS service. Finding
// none is an error, not an empty answer: renderKubelet leaves clusterDNS out of
// a config that has no address, so a pass that happened to see no DNS service
// would publish a DNS-less config to every immutable node — and the group would
// roll it out as if it were a change someone asked for.
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
// reported rather than replaced by defaults: the values decide the node's DNS
// domain and how many pods it advertises, and quietly rendering the defaults
// instead reconfigures every node of every immutable group.
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

// defaultMaxPodsFor is how many pods a node advertises when its NodeGroup asks
// for no particular number: as many as its slice of the pod subnet can address,
// the same brackets bashible uses (candi/bashible/common-steps/all/
// 064_configure_kubelet.sh.tpl). A flat default made every immutable node of a
// /22 cluster advertise 120 against the 500 of every bashible node beside it,
// which is the scheduler skew this number exists to avoid.
//
// The result is capped at what an immutable node's schema accepts, so the widest
// pod subnets get the ceiling rather than a config the API server refuses.
func defaultMaxPodsFor(prefix intstr.IntOrString) int {
	bits := prefix.IntValue()
	if bits == 0 {
		bits = defaultPodSubnetNodeCIDRPrefix
	}
	switch {
	case bits >= 24:
		return maxPodsPerNodeCIDR24
	case bits == 23:
		return maxPodsPerNodeCIDR23
	case bits == 22:
		return min(maxPodsPerNodeCIDR22, maxPodsCeiling)
	default:
		return min(maxPodsPerNodeCIDR21, maxPodsCeiling)
	}
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

	digests := make(map[string]string, 3)

	// The image names carry the version with the separators stripped:
	// containerdSysext224, kubernetesCniSysext162, kubeletSysext1356.
	for prefix, name := range map[string]string{
		"containerdSysext":    containerdExtension,
		"kubernetesCniSysext": cniExtension,
	} {
		d, err := soleDigest(packages, prefix)
		if err != nil {
			return nil, err
		}
		if d != "" {
			digests[name] = d
		}
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

// soleDigest returns the digest of the one image with the given prefix. It
// picks no newest because none can be told: the camelcase name strips the
// separators, so "kubernetesCniSysext1610" is 1.6.10, 1.61.0 and 16.1.0 at
// once. The release ships exactly one of each, so several is a build defect —
// reported, not resolved by guessing. Kept in step with soleDigest in
// dhctl/pkg/immutable/nodeconfig.go, which reads the same digests file.
func soleDigest(packages map[string]string, prefix string) (string, error) {
	found := make([]string, 0, 1)
	for name := range packages {
		suffix, ok := strings.CutPrefix(name, prefix)
		if !ok {
			continue
		}
		// Everything after the prefix is the version, so a non-numeric tail is
		// another image whose name merely starts the same way.
		if _, err := strconv.Atoi(suffix); err != nil {
			continue
		}
		found = append(found, name)
	}

	switch len(found) {
	case 0:
		return "", nil
	case 1:
		return packages[found[0]], nil
	default:
		sort.Strings(found)
		return "", fmt.Errorf("the release carries %d %q system extensions (%s): their names do not say which one is newer",
			len(found), prefix, strings.Join(found, ", "))
	}
}

// newestPatchDigest returns the newest image with the given prefix, which pins
// everything but the patch — so the suffix is one number and compares exactly.
// A string compare would put "kubeletSysext1356" after "kubeletSysext13510",
// i.e. patch 6 over patch 10.
func newestPatchDigest(packages map[string]string, prefix string) string {
	best, bestVer := "", -1
	for name, digest := range packages {
		suffix, ok := strings.CutPrefix(name, prefix)
		if !ok {
			continue
		}
		ver, err := strconv.Atoi(suffix)
		if err != nil {
			continue
		}
		if ver > bestVer {
			best, bestVer = digest, ver
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
