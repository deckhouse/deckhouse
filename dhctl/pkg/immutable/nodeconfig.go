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
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
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

	// The four brackets bashible computes in 064_configure_kubelet.sh.tpl. A
	// master on the flat CRD default would advertise 120 where every bashible
	// node in a /22 cluster advertises 500, and the scheduler believes both.
	maxPodsPodSubnetPrefix24 = 120
	maxPodsPodSubnetPrefix23 = 250
	maxPodsPodSubnetPrefix22 = 500
	maxPodsPodSubnetPrefix21 = 1000

	// maxPodsCeiling is what the nodeConfig schema accepts. node-controller clamps
	// the same ladder to it, so a day-2 render writes the number already there
	// instead of a spec diff on a freshly bootstrapped node.
	maxPodsCeiling = 500

	// defaultPodSubnetNodeCIDRPrefix is what bashible falls back to when the
	// cluster configuration names no prefix.
	defaultPodSubnetNodeCIDRPrefix = 24

	// APIServerPort is where a control-plane node's own kube-apiserver listens.
	APIServerPort = 6443

	nodeGroupLabel = "node.deckhouse.io/group"
	nodeTypeLabel  = "node.deckhouse.io/type"
	cgroupLabel    = "node.deckhouse.io/cgroup"
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
// node. The CA and the token are read from the running cluster rather than
// rendered here: they exist only after Deckhouse creates them, and a second
// source for either is a second source of truth.
type joinInput struct {
	CACert         string
	BootstrapToken string
	// APIServerEndpoints are the apiservers already serving the cluster. The
	// first master leaves a placeholder here and expands it from its own address,
	// which a joining node cannot do: its own apiserver does not exist yet, and
	// will not until control-plane-manager puts one there.
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

	serverTLSBootstrap := false

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
		Kubelet: kubelet{
			// kubelet's feature gates depend on it; without it a DRA workload runs
			// everywhere in the cluster except on this node.
			KubernetesVersion:    kubernetesVersion,
			ClusterDomain:        in.MetaConfig.ClusterDomain,
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
				nodeGroupLabel: masterNodeGroupName,
				nodeTypeLabel:  "CloudPermanent",
				cgroupLabel:    "cgroup2fs",
			},
			// Nobody can approve a serving CSR until Deckhouse is installed,
			// and kubelet blocks on it. bashible turns it off on the first
			// master for the same reason.
			ServerTLSBootstrap:  &serverTLSBootstrap,
			ResourceReservation: &resourceReservation{Mode: "Auto"},
		},
		ContainerRuntime: containerRuntime{
			SandboxImage:           pauseImage,
			MaxConcurrentDownloads: defaultMaxConcurrentDownloads,
		},
		// The zeroth master is its own apiserver and its address is unknown
		// while the payload is being built, so the endpoint stays a
		// placeholder; the node expands it from its own address on load.
		APIServerEndpoints: []string{fmt.Sprintf("https://$MY_IP:%d", APIServerPort)},
		UpdatePolicy:       updatePolicy{Mode: "Automatic"},
		Registry:           registry,
	}

	if in.MetaConfig.ClusterDNSAddress != "" {
		spec.Kubelet.ClusterDNS = []string{in.MetaConfig.ClusterDNSAddress}
	}

	if in.Join != nil {
		// Everything a node needs to enter a cluster that already runs, and the
		// three fields above that only made sense for the one that starts it.
		spec.Kubelet.CACert = in.Join.CACert
		spec.Kubelet.BootstrapToken = in.Join.BootstrapToken
		spec.APIServerEndpoints = in.Join.APIServerEndpoints
		// Deckhouse is running by now, so the serving CSR gets approved and
		// kubelet does not block on it. Left off, this node would be the only one
		// in the cluster with a self-signed serving certificate — no kubectl exec
		// and no kubectl logs against it, for the life of the cluster.
		spec.Kubelet.ServerTLSBootstrap = nil
	}

	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf(
		"Built nodeConfig for %s: Kubernetes %s, %d system extensions, join=%t",
		in.NodeName, kubernetesVersion, len(extensions), in.Join != nil,
	))

	return &nodeConfig{
		APIVersion: payloadAPIVersion,
		Kind:       nodeConfigKind,
		Metadata: objectMeta{
			Name:   in.NodeName,
			Labels: map[string]string{nodeGroupLabel: masterNodeGroupName},
		},
		Spec: spec,
	}, nil
}

// SysextDigests resolves the digests of the three system extensions an immutable
// node runs, from the map baked into the installer image. Pure; the context is
// here for the package's uniform exported signature.
func SysextDigests(_ context.Context, metaConfig *config.MetaConfig) (map[string]string, error) {
	version, err := kubernetesVersion(metaConfig)
	if err != nil {
		return nil, err
	}
	return sysextDigests(metaConfig.Images.ConvertToMap(), version)
}

// maxPods mirrors the number bashible gives kubelet in
// 064_configure_kubelet.sh.tpl — the scheduler believes it as the node's
// capacity, so a master that disagrees with the fleet skews placement.
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

	bracket := maxPodsPodSubnetPrefix21
	switch {
	case prefix >= 24:
		bracket = maxPodsPodSubnetPrefix24
	case prefix == 23:
		bracket = maxPodsPodSubnetPrefix23
	case prefix == 22:
		bracket = maxPodsPodSubnetPrefix22
	}

	return min(bracket, maxPodsCeiling), nil
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

func sysextExtensions(images map[string]any, kubernetesVersion string) ([]extension, error) {
	digests, err := sysextDigests(images, kubernetesVersion)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(digests))
	for name := range digests {
		names = append(names, name)
	}
	sort.Strings(names)

	extensions := make([]extension, 0, len(names))
	for _, name := range names {
		extensions = append(extensions, extension{
			Name:        name,
			Digest:      digests[name],
			RequestedBy: platformExtensionRequestedBy,
		})
	}
	return extensions, nil
}

// sysextDigests looks the extensions up in images_digests.json. The image names
// there are produced by the sprig camelcase function, which strips the
// separators: kubelet-sysext-1-34-9 becomes registrypackages.kubeletSysext1349.
func sysextDigests(images map[string]any, kubernetesVersion string) (map[string]string, error) {
	packages, err := digestGroup(images, registryPackagesDigestsKey)
	if err != nil {
		return nil, err
	}

	minor := strings.ReplaceAll(kubernetesVersion, ".", "")

	containerd, err := soleDigest(packages, "containerdSysext")
	if err != nil {
		return nil, err
	}
	cni, err := soleDigest(packages, "kubernetesCniSysext")
	if err != nil {
		return nil, err
	}

	digests := map[string]string{
		containerdExtension: containerd,
		cniExtension:        cni,
		kubeletExtension:    newestPatchDigest(packages, "kubeletSysext"+minor),
	}

	for _, name := range []string{containerdExtension, cniExtension, kubeletExtension} {
		if digests[name] == "" {
			return nil, fmt.Errorf(
				"the installer image carries no %q system extension digest for Kubernetes %s",
				name, kubernetesVersion,
			)
		}
	}

	return digests, nil
}

// soleDigest returns the digest of the one image with the given prefix. It picks
// no newest because none can be told: camelcase strips the separators, so
// "kubernetesCniSysext1610" is 1.6.10, 1.61.0 and 16.1.0 at once.
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
	for name, digest := range packages {
		suffix, ok := strings.CutPrefix(name, prefix)
		if !ok {
			continue
		}
		patch, err := strconv.Atoi(suffix)
		if err != nil {
			continue
		}
		if patch > bestPatch {
			best, bestPatch = digest, patch
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

// etcdDataMountName is both the mount's name and the label the node writes on
// the filesystem it makes, so the disk is recognisable as this one afterwards.
// Capped at the ext4 label size, which is 16 characters.
const etcdDataMountName = "kubernetes-data"

// etcdDataDir is where the etcd static pod expects its data. The path is in the
// control-plane manifest as a hostPath, so it is not the node's to choose.
const etcdDataDir = "/var/lib/etcd"

// etcdDataMode is what etcd checks on every start. A freshly made ext4 has its
// root at 0755 and etcd refuses to run on that.
const etcdDataMode = "0700"

// etcdDiskSize is the smallest disk a cloud installation is ever given for etcd.
// A size with no operator means "at least this much", so it also covers the
// providers that hand out 15 or 20 gibibytes, and it rules out the config drives
// and other small volumes a machine comes with.
const etcdDiskSize = "10Gi"

// systemDiskSize tells the initramfs which disk to install onto.
//
// The threshold sits between the two disks a master gets — 10Gi for etcd and 50Gi
// for the system — rather than at either one. Naming the system disk's own size
// would make the match depend on the provider rounding it the way we expect, and
// a disk handed out as 50Gi arriving as 49.9 would leave the node with nothing to
// install onto.
//
// Order cannot be used instead: the machine attaches the system disk first, but
// the kernel named it /dev/sdc while the 10Gi one became /dev/sdb (measured on
// DVP, 10.08.2026). Size is what actually separates them.
const systemDiskSize = ">=20Gi"

// etcdMounts gives a control-plane node the disk etcd lives on.
//
// The disk is described rather than named: dhctl renders this document before
// the machine exists, so there is no /dev path and no by-id link to point at
// yet. What is known is what was asked of the provider — one spare disk, blank,
// at least etcdDiskSize — and that is what the selector says.
//
// A machine with no second disk matches nothing, which is a supported way to
// run: etcd shares the data partition, slower but correct. The node says so in
// its log rather than failing.
func etcdMounts() []mount {
	return []mount{{
		Name: etcdDataMountName,
		PartitionSelector: &partitionSelector{
			Size: etcdDiskSize,
			// Without this the selector would see partitions only, and the disk a
			// cloud attaches has none: no partition table, no filesystem.
			Blank: true,
		},
		BindTo: etcdDataDir,
		Mode:   etcdDataMode,
	}}
}

func clusterConfigString(metaConfig *config.MetaConfig, key string) (string, error) {
	raw, ok := metaConfig.ClusterConfig[key]
	if !ok || len(raw) == 0 {
		return "", nil
	}

	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("parse %s from the cluster configuration: %w", key, err)
	}
	return value, nil
}
