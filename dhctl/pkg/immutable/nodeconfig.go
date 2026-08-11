// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package immutable

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
)

const (
	// containerdExtension, cniExtension and kubeletExtension are the system
	// extensions every immutable node runs.
	containerdExtension = "containerd"
	cniExtension        = "kubernetes-cni"
	kubeletExtension    = "kubelet"

	// platformExtensionRequestedBy names the module that wants the extension, not
	// the process that wrote the file — which is why it stays "node-manager" when
	// node-controller re-renders this node.
	platformExtensionRequestedBy = "node-manager"

	// registryPackagesDigestsKey is the images_digests.json module the sysext
	// images are built in.
	registryPackagesDigestsKey = "registrypackages"

	// commonDigestsKey and pauseImageName locate the sandbox image. The
	// registry.k8s.io default of the nodeConfig CRD is unreachable from a node
	// that only talks to the Deckhouse registry.
	commonDigestsKey = "common"
	pauseImageName   = "pause"

	// osImageNameAndTag is the olcedar image the node boots from, pinned by tag
	// the way node-controller pins it for day-2 nodes. Only the name and tag are
	// constant; the repository comes from the configured registry.
	osImageNameAndTag = "olcedar:v0.1"

	// Defaults mirroring the nodeConfig CRD field defaults. They are applied
	// here because the bootstrap payload is marshalled to a file instead of
	// being created through the API server, where CRD defaulting runs.
	defaultContainerLogMaxSize    = "50Mi"
	defaultContainerLogMaxFiles   = 4
	defaultMaxConcurrentDownloads = 3

	// defaultPodSubnetNodeCIDRPrefix is what bashible falls back to when the
	// cluster configuration names no prefix.
	defaultPodSubnetNodeCIDRPrefix = 24

	// APIServerPort is where a control-plane node's own kube-apiserver listens.
	APIServerPort = 6443

	nodeTypeLabel = "node.deckhouse.io/type"
	cgroupLabel   = "node.deckhouse.io/cgroup"
)

// nodeConfigInput is everything buildNodeConfig needs.
type nodeConfigInput struct {
	// NodeName is the name the node registers under.
	NodeName string
	// MetaConfig is the parsed cluster configuration.
	MetaConfig *config.MetaConfig
	// Join carries what a node needs to enter a cluster that already runs. It is
	// nil for the first master, which has no cluster to join.
	Join *joinInput
}

// joinInput is what the cluster — not the installer — decides for a joining
// node. The CA and the token are read from the running cluster: a second
// source for either would be a second source of truth.
type joinInput struct {
	CACert         string
	BootstrapToken string
	// APIServerEndpoints are the apiservers already serving the cluster. A
	// joining node cannot use the first master's placeholder trick: its own
	// apiserver does not exist until control-plane-manager puts one there.
	APIServerEndpoints []string
}

// buildNodeConfig renders the nodeConfig the first control-plane node boots
// with. It differs from a worker's config in a few places that only hold for
// the zeroth master; each of them is commented at its field.
func buildNodeConfig(ctx context.Context, in nodeConfigInput) (*nodeConfig, error) {
	if in.NodeName == "" {
		return nil, errors.New("build node config: node name is empty")
	}
	if in.MetaConfig == nil {
		return nil, errors.New("build node config: meta config is nil")
	}

	kubernetesVersion, err := kubernetesVersion(in.MetaConfig)
	if err != nil {
		return nil, err
	}

	images := in.MetaConfig.Images.ConvertToMap()

	extensions, err := sysextExtensions(images, kubernetesVersion)
	if err != nil {
		return nil, err
	}

	registry, err := nodeRegistry(in.MetaConfig)
	if err != nil {
		return nil, err
	}

	pauseImage, err := sandboxImage(registry, images)
	if err != nil {
		return nil, err
	}

	podsPerNode, err := maxPods(in.MetaConfig)
	if err != nil {
		return nil, err
	}

	spec := nodeSpec{
		NodeName: in.NodeName,
		OSImage:  registry.Address + registry.Path + "/" + osImageNameAndTag,
		Storage: storage{
			DiskSelector: &diskSelector{Size: systemDiskSize},
			Mounts:       etcdMounts(),
		},
		Extensions: extensions,
		Kernel: kernel{
			Sysctl: map[string]string{
				// The same four node-controller renders for day-2 nodes; kernel.panic*
				// comes from candi/bashible/common-steps/all/041_configure_sysctl_tuner.sh.tpl
				// at its non-fencing value.
				"net.ipv4.ip_forward": "1",
				"vm.max_map_count":    "262144",
				// kubelet refuses to start without these (protect-kernel-defaults).
				"kernel.panic":         "10",
				"kernel.panic_on_oops": "1",
			},
		},
		Network: network{
			Hostname:   in.NodeName,
			Interfaces: []networkInterface{{Name: "eth0", DHCP: true}},
		},
		Kubelet: nodeKubelet(in.MetaConfig, kubernetesVersion, podsPerNode),
		ContainerRuntime: containerRuntime{
			SandboxImage:           pauseImage,
			MaxConcurrentDownloads: defaultMaxConcurrentDownloads,
		},
		// The zeroth master is its own apiserver and its address is unknown
		// while the payload is being built, so the endpoint stays a
		// placeholder; the node expands it from its own address on load.
		APIServerEndpoints: []string{fmt.Sprintf("https://%s:%d", nodeAddressPlaceholder, APIServerPort)},
		UpdatePolicy:       updatePolicy{Mode: "Automatic"},
		Registry:           registry,
	}

	applyJoinToSpec(&spec, in.Join)

	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf(
		"Built nodeConfig for %s: Kubernetes %s, %d system extensions, join=%t",
		in.NodeName, kubernetesVersion, len(extensions), in.Join != nil,
	))

	return &nodeConfig{
		APIVersion: payloadAPIVersion,
		Kind:       nodeConfigKind,
		Metadata: objectMeta{
			Name:   in.NodeName,
			Labels: map[string]string{global.NodeGroupLabel: global.MasterNodeGroupName},
		},
		Spec: spec,
	}, nil
}

// nodeKubelet is the kubelet section of a control-plane node's config, in the
// shape the node that starts the cluster needs it; applyJoinToSpec turns it into
// what a node joining a running cluster needs.
func nodeKubelet(metaConfig *config.MetaConfig, kubernetesVersion string, podsPerNode int) kubelet {
	serverTLSBootstrap := false

	k := kubelet{
		// kubelet's feature gates depend on it; without it a DRA workload runs
		// everywhere in the cluster except on this node.
		KubernetesVersion:    kubernetesVersion,
		ClusterDomain:        metaConfig.ClusterDomain,
		MaxPods:              podsPerNode,
		ContainerLogMaxSize:  defaultContainerLogMaxSize,
		ContainerLogMaxFiles: defaultContainerLogMaxFiles,
		// The node is CAPI-backed, so the cloud-controller-manager has to
		// assign its providerID before CAPI can match Machine to Node.
		ExternalCloudProvider: true,
		// Only labels kubelet may set on itself: NodeRestriction rejects
		// node-role.kubernetes.io/*, and a rejected registration means the node
		// never joins. The role label and taint come later, from the node.
		NodeLabels: map[string]string{
			global.NodeGroupLabel: global.MasterNodeGroupName,
			nodeTypeLabel:         "CloudPermanent",
			cgroupLabel:           "cgroup2fs", // olcedar's only layout; bashible probes it in 092_set_cgroup_type.sh.tpl
		},
		// Nobody can approve a serving CSR until Deckhouse is installed, and
		// kubelet blocks on it. bashible does the same on the first master
		// (candi/bashible/common-steps/all/064_configure_kubelet.sh.tpl).
		ServerTLSBootstrap:  &serverTLSBootstrap,
		ResourceReservation: &resourceReservation{Mode: "Auto"},
	}

	if metaConfig.ClusterDNSAddress != "" {
		k.ClusterDNS = []string{metaConfig.ClusterDNSAddress}
	}

	return k
}

// applyJoinToSpec adds what a node needs to enter a cluster that already runs,
// and drops the three fields that only made sense for the one that starts it.
func applyJoinToSpec(spec *nodeSpec, join *joinInput) {
	if join == nil {
		return
	}

	spec.Kubelet.CACert = join.CACert
	spec.Kubelet.BootstrapToken = join.BootstrapToken
	spec.APIServerEndpoints = join.APIServerEndpoints
	// Deckhouse is running by now, so the serving CSR gets approved. Left off,
	// this node would keep a self-signed serving certificate — no kubectl exec or
	// logs against it, for the life of the cluster.
	spec.Kubelet.ServerTLSBootstrap = nil
}

// ValidateSysext reports whether the installer image carries the system
// extensions an immutable node runs; a preflight check calls it to fail early.
// Pure; the context is here for the package's uniform exported signature.
func ValidateSysext(_ context.Context, metaConfig *config.MetaConfig) error {
	version, err := kubernetesVersion(metaConfig)
	if err != nil {
		return err
	}
	_, err = sysextExtensions(metaConfig.Images.ConvertToMap(), version)
	return err
}

// maxPods mirrors the ladder bashible computes in
// candi/bashible/common-steps/all/064_configure_kubelet.sh.tpl — the scheduler
// believes it as capacity, so a master off the fleet's number skews placement.
func maxPods(metaConfig *config.MetaConfig) (int, error) {
	clusterConfig, err := metaConfig.ClusterConfigMap()
	if err != nil {
		return 0, fmt.Errorf("read the cluster configuration: %w", err)
	}

	prefix := defaultPodSubnetNodeCIDRPrefix
	if raw, ok := clusterConfig["podSubnetNodeCIDRPrefix"].(string); ok {
		if parsed, err := strconv.Atoi(raw); err == nil {
			prefix = parsed
		}
	}

	// A prefix outside the ladder takes the nearest step, as the template's
	// ge/le branches do. The top step is also what the nodeConfig schema accepts
	// as its maximum, so the ladder needs no further clamp.
	byPrefix := map[int]int{24: 120, 23: 250, 22: 500, 21: 1000}

	return byPrefix[min(max(prefix, 21), 24)], nil
}

// kubernetesVersion is the cluster's Kubernetes minor version with "Automatic"
// already resolved to the installer default.
func kubernetesVersion(metaConfig *config.MetaConfig) (string, error) {
	clusterConfig, err := metaConfig.ClusterConfigMap()
	if err != nil {
		return "", fmt.Errorf("read the cluster configuration: %w", err)
	}

	version, _ := clusterConfig["kubernetesVersion"].(string)
	if version == "" {
		return "", errors.New("kubernetesVersion is empty in the cluster configuration")
	}
	return version, nil
}

// sysextExtensions looks the extensions up in images_digests.json. The image
// names there are produced by the sprig camelcase function, which strips the
// separators: kubelet-sysext-1-34-9 becomes registrypackages.kubeletSysext1349.
func sysextExtensions(images map[string]any, kubernetesVersion string) ([]extension, error) {
	packages, err := digestGroup(images, registryPackagesDigestsKey)
	if err != nil {
		return nil, err
	}

	containerd, err := soleDigest(packages, "containerdSysext")
	if err != nil {
		return nil, err
	}
	cni, err := soleDigest(packages, "kubernetesCniSysext")
	if err != nil {
		return nil, err
	}
	minor := strings.ReplaceAll(kubernetesVersion, ".", "")

	// The order is the payload's: the node writes them in it, and every rendered
	// document since the first one has carried them this way.
	extensions := []extension{
		{Name: containerdExtension, Digest: containerd, RequestedBy: platformExtensionRequestedBy},
		{Name: kubeletExtension, Digest: newestPatchDigest(packages, "kubeletSysext"+minor), RequestedBy: platformExtensionRequestedBy},
		{Name: cniExtension, Digest: cni, RequestedBy: platformExtensionRequestedBy},
	}
	for _, e := range extensions {
		if e.Digest == "" {
			return nil, fmt.Errorf(
				"the installer image carries no %q system extension digest for Kubernetes %s",
				e.Name, kubernetesVersion,
			)
		}
	}

	return extensions, nil
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

// soleDigest returns the digest of the one image with the given prefix. No
// "newest" can be told: camelcase strips separators, so "kubernetesCniSysext1610"
// is ambiguous. Kept in step with node-controller's nodeconfig/sources.go.
func soleDigest(packages map[string]string, prefix string) (string, error) {
	found := slices.Sorted(maps.Keys(versionedImages(packages, prefix)))

	switch len(found) {
	case 0:
		return "", nil
	case 1:
		return packages[found[0]], nil
	default:
		return "", fmt.Errorf(
			"the installer image carries %d %q system extensions (%s): their names do not say which one is newer. "+
				"That is a defect in the installer image, which is built to ship exactly one of each; "+
				"nothing in the cluster configuration causes it and nothing in it can work around it",
			len(found), prefix, strings.Join(found, ", "),
		)
	}
}

// newestPatchDigest returns the newest image with the given prefix, which pins
// everything but the patch — so the suffix is one number and compares exactly.
// A string compare would put "kubeletSysext1356" after "kubeletSysext13510".
func newestPatchDigest(packages map[string]string, prefix string) string {
	best, bestPatch := "", -1
	for name, patch := range versionedImages(packages, prefix) {
		if patch > bestPatch {
			best, bestPatch = packages[name], patch
		}
	}
	return best
}

// sandboxImage pins the pause image to the cluster's own registry: the CRD's
// registry.k8s.io default is unreachable from the node. It takes the registry
// the document carries, so the reference and spec.registry cannot disagree.
func sandboxImage(registry *registrySpec, images map[string]any) (string, error) {
	common, err := digestGroup(images, commonDigestsKey)
	if err != nil {
		return "", err
	}

	digest := common[pauseImageName]
	if digest == "" {
		return "", fmt.Errorf("the installer image carries no %q.%q digest", commonDigestsKey, pauseImageName)
	}

	// One repository for every Deckhouse image: the group a digest is filed under
	// is a map key, not a path segment, and appending it yields a 404 that
	// surfaces as a pod sandbox nobody can create.
	return registry.Address + registry.Path + "@" + digest, nil
}

// nodeRegistry gives the node direct registry access: it pulls the
// control-plane images and the system extensions itself, with no in-cluster
// registry-packages-proxy to go through during bootstrap.
func nodeRegistry(metaConfig *config.MetaConfig) (*registrySpec, error) {
	settings := metaConfig.Registry.Settings
	if settings.Mode != constant.ModeUnmanaged {
		return nil, fmt.Errorf("registry mode %q is not supported for an immutable master, use %q", settings.Mode, constant.ModeUnmanaged)
	}

	address, path := settings.RemoteData.AddressAndPath()
	if address == "" {
		return nil, errors.New("registry address is empty")
	}

	return &registrySpec{
		Address: address,
		Path:    path,
		Scheme:  string(settings.RemoteData.Scheme),
		CA:      settings.RemoteData.CA,
		Auth:    settings.RemoteData.AuthBase64(),
	}, nil
}

func digestGroup(images map[string]any, key string) (map[string]string, error) {
	raw, ok := images[key].(map[string]any)
	if !ok || len(raw) == 0 {
		return nil, fmt.Errorf("the installer image carries no %q image digests", key)
	}

	group := make(map[string]string, len(raw))
	for name, digest := range raw {
		if value, ok := digest.(string); ok {
			group[name] = value
		}
	}
	return group, nil
}

// systemDiskSize tells the initramfs which disk to install onto. The threshold
// sits between the etcd (10Gi) and system (50Gi) disks: exact sizes depend on
// provider rounding, and device names do not follow attach order.
const systemDiskSize = ">=20Gi"

// etcdMounts gives a control-plane node the disk etcd lives on. The disk is
// described, not named — no /dev path exists before the machine does. No second
// disk matches nothing, which is supported: etcd shares the data partition.
func etcdMounts() []mount {
	return []mount{{
		// The name is also the label the node writes on the filesystem it makes,
		// so it is capped at the ext4 label size of 16 characters.
		Name: "kubernetes-data",
		PartitionSelector: &partitionSelector{
			// The smallest disk a cloud installation is ever given for etcd. A size
			// with no operator means "at least this much", so larger disks match
			// too while config drives and other small volumes do not.
			Size: "10Gi",
			// Without this the selector would see partitions only, and the disk a
			// cloud attaches has none: no partition table, no filesystem.
			Blank: true,
		},
		// Where the etcd static pod expects its data: the path is a hostPath in
		// the control-plane manifest, so it is not the node's to choose.
		BindTo: "/var/lib/etcd",
		// What etcd checks on every start. A freshly made ext4 has its root at
		// 0755 and etcd refuses to run on that.
		Mode: "0700",
	}}
}
