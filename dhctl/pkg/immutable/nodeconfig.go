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
	"fmt"
	"sort"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

const (
	// containerdExtension, cniExtension and kubeletExtension are the system
	// extensions every immutable node runs.
	containerdExtension = "containerd"
	cniExtension        = "kubernetes-cni"
	kubeletExtension    = "kubelet"

	// registryPackagesDigestsKey is the images_digests.json module the sysext
	// images are built in.
	registryPackagesDigestsKey = "registrypackages"

	// commonDigestsKey and pauseImageName locate the sandbox image. The
	// registry.k8s.io default of the NodeConfig CRD is unreachable from a node
	// that only talks to the Deckhouse registry.
	commonDigestsKey = "common"
	pauseImageName   = "pause"

	// defaultOSImage mirrors the pinned image node-controller renders for
	// day-2 nodes (internal/controller/nodeconfig/constants.go).
	defaultOSImage = "registry.deckhouse.io/deckhouse/olcedar:v0.1"

	// Defaults mirroring the NodeConfig CRD field defaults. They are applied
	// here because the bootstrap payload is marshalled to a file instead of
	// being created through the API server, where CRD defaulting runs.
	defaultMaxPods                = 110
	defaultContainerLogMaxSize    = "50Mi"
	defaultContainerLogMaxFiles   = 4
	defaultMaxConcurrentDownloads = 3

	nodeGroupLabel = "node.deckhouse.io/group"
	nodeTypeLabel  = "node.deckhouse.io/type"
	cgroupLabel    = "node.deckhouse.io/cgroup"
)

// NodeConfigInput is everything BuildNodeConfig needs.
type NodeConfigInput struct {
	// NodeName is the name the first master registers under.
	NodeName string
	// MetaConfig is the parsed cluster configuration.
	MetaConfig *config.MetaConfig
}

// BuildNodeConfig renders the NodeConfig the first control-plane node boots
// with. It differs from a worker's config in a few places that only hold for
// the zeroth master; each of them is commented at its field.
func BuildNodeConfig(ctx context.Context, in NodeConfigInput) (*NodeConfig, error) {
	if in.NodeName == "" {
		return nil, fmt.Errorf("build node config: node name is empty")
	}
	if in.MetaConfig == nil {
		return nil, fmt.Errorf("build node config: meta config is nil")
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

	sandboxImage, err := sandboxImage(in.MetaConfig, images)
	if err != nil {
		return nil, err
	}

	systemDisk, _, err := MasterDisks(in.MetaConfig)
	if err != nil {
		return nil, err
	}

	serverTLSBootstrap := false

	spec := NodeSpec{
		NodeName:   in.NodeName,
		OSImage:    defaultOSImage,
		Storage:    systemDisk,
		Extensions: extensions,
		Kernel: Kernel{
			Sysctl: map[string]string{
				"net.ipv4.ip_forward": "1",
				"vm.max_map_count":    "262144",
				// kubelet refuses to start without these (protect-kernel-defaults).
				"kernel.panic":         "10",
				"kernel.panic_on_oops": "1",
			},
		},
		Network: Network{
			Hostname:   in.NodeName,
			Interfaces: []NetworkInterface{{Name: "eth0", DHCP: true}},
		},
		Kubelet: Kubelet{
			ClusterDomain:        in.MetaConfig.ClusterDomain,
			MaxPods:              defaultMaxPods,
			ContainerLogMaxSize:  defaultContainerLogMaxSize,
			ContainerLogMaxFiles: defaultContainerLogMaxFiles,
			// The node is CAPI-backed, so the cloud-controller-manager has to
			// assign its providerID before CAPI can match Machine to Node.
			ExternalCloudProvider: true,
			// Only labels kubelet is allowed to set on itself: NodeRestriction
			// rejects node-role.kubernetes.io/*, and a rejected registration
			// means the node never joins. The control-plane role label and its
			// taint are added by the node itself once its apiserver answers.
			NodeLabels: map[string]string{
				nodeGroupLabel: MasterNodeGroupName,
				nodeTypeLabel:  "CloudPermanent",
				cgroupLabel:    "cgroup2fs",
			},
			// Nobody can approve a serving CSR until Deckhouse is installed,
			// and kubelet blocks on it. bashible turns it off on the first
			// master for the same reason.
			ServerTLSBootstrap:  &serverTLSBootstrap,
			ResourceReservation: &ResourceReservation{Mode: "Auto"},
		},
		ContainerRuntime: ContainerRuntime{
			SandboxImage:           sandboxImage,
			MaxConcurrentDownloads: defaultMaxConcurrentDownloads,
		},
		// The zeroth master is its own apiserver and its address is unknown
		// while the payload is being built, so the endpoint stays a
		// placeholder; the node expands it from its own address on load.
		APIServerEndpoints: []string{"https://$MY_IP:6443"},
		UpdatePolicy:       UpdatePolicy{Mode: "Automatic"},
		Registry:           registry,
	}

	if in.MetaConfig.ClusterDNSAddress != "" {
		spec.Kubelet.ClusterDNS = []string{in.MetaConfig.ClusterDNSAddress}
	}

	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf(
		"Built NodeConfig for %s: Kubernetes %s, %d system extensions", in.NodeName, kubernetesVersion, len(extensions),
	))

	return &NodeConfig{
		APIVersion: PayloadAPIVersion,
		Kind:       NodeConfigKind,
		Metadata: ObjectMeta{
			Name:   in.NodeName,
			Labels: map[string]string{nodeGroupLabel: MasterNodeGroupName},
		},
		Spec: spec,
	}, nil
}

// MasterDisks builds the disk layout of the master VM: the system disk that
// goes into NodeConfig and the control-plane state disk that goes into
// ControlPlaneConfig.
//
// The VM has three block devices: the root disk, the etcd disk and the
// cloud-init CDROM. Their serials are only known after the infrastructure is
// created, but the payload has to exist before that, so the two data disks are
// told apart by size: everything at or above the midpoint between the two
// configured sizes is the root disk, everything at or below it is the etcd
// disk. The cloud-init CDROM is small enough to match the etcd selector too;
// the initramfs filters it out by filesystem type.
//
// It doubles as the preflight check for the master's disk configuration: it
// fails when the etcd disk is missing or not smaller than the root disk,
// because the selectors cannot tell the disks apart in either case.
func MasterDisks(metaConfig *config.MetaConfig) (Disk, Disk, error) {
	rootSize, etcdSize, err := masterDiskSizes(metaConfig)
	if err != nil {
		return Disk{}, Disk{}, err
	}

	root, err := resource.ParseQuantity(rootSize)
	if err != nil {
		return Disk{}, Disk{}, fmt.Errorf("parse masterNodeGroup.instanceClass.rootDisk.size %q: %w", rootSize, err)
	}
	etcd, err := resource.ParseQuantity(etcdSize)
	if err != nil {
		return Disk{}, Disk{}, fmt.Errorf("parse masterNodeGroup.instanceClass.etcdDisk.size %q: %w", etcdSize, err)
	}

	if etcd.Cmp(root) >= 0 {
		return Disk{}, Disk{}, fmt.Errorf(
			"masterNodeGroup.instanceClass.etcdDisk.size (%s) must be smaller than rootDisk.size (%s): "+
				"an immutable master picks its disks by size before the VM exists, and disks of the same or inverted size cannot be told apart",
			etcdSize, rootSize,
		)
	}

	midpoint := resource.NewQuantity((root.Value()+etcd.Value())/2, resource.BinarySI).String()

	systemDisk := Disk{DiskSelector: &DiskSelector{Size: ">=" + midpoint}}
	controlPlaneDisk := Disk{DiskSelector: &DiskSelector{Size: "<=" + midpoint}}

	return systemDisk, controlPlaneDisk, nil
}

func masterDiskSizes(metaConfig *config.MetaConfig) (string, string, error) {
	raw, ok := metaConfig.ProviderClusterConfig["masterNodeGroup"]
	if !ok || len(raw) == 0 {
		return "", "", fmt.Errorf("masterNodeGroup is missing from the provider cluster configuration")
	}

	var group struct {
		InstanceClass struct {
			RootDisk *struct {
				Size string `json:"size"`
			} `json:"rootDisk"`
			EtcdDisk *struct {
				Size string `json:"size"`
			} `json:"etcdDisk"`
		} `json:"instanceClass"`
	}
	if err := json.Unmarshal(raw, &group); err != nil {
		return "", "", fmt.Errorf("parse masterNodeGroup from the provider cluster configuration: %w", err)
	}

	if group.InstanceClass.RootDisk == nil || group.InstanceClass.RootDisk.Size == "" {
		return "", "", fmt.Errorf("masterNodeGroup.instanceClass.rootDisk.size is not set")
	}
	if group.InstanceClass.EtcdDisk == nil || group.InstanceClass.EtcdDisk.Size == "" {
		return "", "", fmt.Errorf("masterNodeGroup.instanceClass.etcdDisk.size is not set: an immutable master keeps etcd data on a separate disk")
	}

	return group.InstanceClass.RootDisk.Size, group.InstanceClass.EtcdDisk.Size, nil
}

// SysextDigests resolves the digests of the three system extensions an
// immutable node runs from the digests baked into the installer image.
func SysextDigests(metaConfig *config.MetaConfig) (map[string]string, error) {
	version, err := kubernetesVersion(metaConfig)
	if err != nil {
		return nil, err
	}
	return sysextDigests(metaConfig.Images.ConvertToMap(), version)
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
		return "", fmt.Errorf("kubernetesVersion is empty in the cluster configuration")
	}
	return version, nil
}

func sysextExtensions(images map[string]any, kubernetesVersion string) ([]Extension, error) {
	digests, err := sysextDigests(images, kubernetesVersion)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(digests))
	for name := range digests {
		names = append(names, name)
	}
	sort.Strings(names)

	extensions := make([]Extension, 0, len(names))
	for _, name := range names {
		extensions = append(extensions, Extension{
			Name:        name,
			Digest:      digests[name],
			RequestedBy: "dhctl",
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

	digests := map[string]string{
		containerdExtension: newestDigest(packages, "containerdSysext"),
		cniExtension:        newestDigest(packages, "kubernetesCniSysext"),
		kubeletExtension:    newestDigest(packages, "kubeletSysext"+minor),
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

// newestDigest returns the digest of the newest image with the given prefix.
// The version suffix is compared numerically: once a component reaches two
// digits a string compare picks the wrong image ("kubeletSysext1356" sorts
// after "kubeletSysext13510", i.e. patch 6 over patch 10).
func newestDigest(packages map[string]string, prefix string) string {
	best, bestVersion := "", -1
	for name, digest := range packages {
		suffix, ok := strings.CutPrefix(name, prefix)
		if !ok {
			continue
		}
		version, err := strconv.Atoi(suffix)
		if err != nil {
			continue
		}
		if version > bestVersion {
			best, bestVersion = digest, version
		}
	}
	return best
}

// sandboxImage pins the pause image to the cluster's own registry: the
// registry.k8s.io default of the NodeConfig CRD is unreachable from a node that
// only talks to the Deckhouse registry.
func sandboxImage(metaConfig *config.MetaConfig, images map[string]any) (string, error) {
	common, err := digestGroup(images, commonDigestsKey)
	if err != nil {
		return "", err
	}

	digest := common[pauseImageName]
	if digest == "" {
		return "", fmt.Errorf("the installer image carries no %q.%q digest", commonDigestsKey, pauseImageName)
	}

	imagesRepo := metaConfig.Registry.Settings.RemoteData.ImagesRepo
	if imagesRepo == "" {
		return "", fmt.Errorf("registry imagesRepo is empty")
	}

	return fmt.Sprintf("%s/%s@%s", imagesRepo, commonDigestsKey, digest), nil
}

// nodeRegistry gives the node direct registry access: it pulls the
// control-plane images and the system extensions itself, with no in-cluster
// registry-packages-proxy to go through during bootstrap.
func nodeRegistry(metaConfig *config.MetaConfig) (*Registry, error) {
	settings := metaConfig.Registry.Settings
	if settings.Mode != constant.ModeUnmanaged {
		return nil, fmt.Errorf("registry mode %q is not supported for an immutable master, use %q", settings.Mode, constant.ModeUnmanaged)
	}

	address, path := settings.RemoteData.AddressAndPath()
	if address == "" {
		return nil, fmt.Errorf("registry address is empty")
	}

	return &Registry{
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
