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

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
)

// What dhctl fills the NodeConfig with when the cluster configuration names
// nothing. The names the node is addressed by live in constants.go.
const (
	// osImageName is the olcedar image in images_digests.json. It is delivered by
	// digest, not by tag: the node records the digest at install and decides a
	// rootfs update by comparing it, which a moving tag makes impossible.
	nodeManagerDigestsKey = "nodeManager"
	osImageName           = "olcedar"

	// systemDiskSize tells the initramfs which disk to install onto: a threshold
	// between the etcd (10Gi) and system (50Gi) disks, because provider rounding
	// moves the exact sizes and device names do not follow attach order.
	systemDiskSize = ">=20Gi"

	// The +kubebuilder:default values of the NodeConfig CRD
	// (modules/040-node-manager/images/node-controller/src/api/internal.deckhouse.io/v1alpha1/nodeconfig_types.go),
	// applied here because the payload is marshalled to a file instead of being
	// created through the API server, where CRD defaulting runs.
	defaultContainerLogMaxSize    = "50Mi"
	defaultContainerLogMaxFiles   = 4
	defaultMaxConcurrentDownloads = 3

	// defaultPodSubnetNodeCIDRPrefix is what bashible falls back to when the
	// cluster configuration names no prefix.
	defaultPodSubnetNodeCIDRPrefix = 24
)

type nodeConfigInput struct {
	NodeName   string
	MetaConfig *config.MetaConfig
	// Join carries what the cluster — not the installer — decides for a node
	// entering a cluster that already runs. It is nil for the first master,
	// which has no cluster to join.
	Join *JoinPayloadInput
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

	clusterConfig, err := in.MetaConfig.ClusterConfigMap()
	if err != nil {
		return nil, fmt.Errorf("read the cluster configuration: %w", err)
	}

	spec, err := buildNodeSpec(in, clusterConfig)
	if err != nil {
		return nil, err
	}

	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf(
		"Built nodeConfig for %s: Kubernetes %s, %d system extensions, join=%t",
		in.NodeName, spec.Kubelet.KubernetesVersion, len(spec.Extensions), in.Join != nil,
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

// buildNodeSpec assembles what the node is told to be. Everything in it holds
// for any control-plane node; the fields that only hold for the zeroth master
// are commented where they are set.
func buildNodeSpec(in nodeConfigInput, clusterConfig map[string]any) (nodeSpec, error) {
	kubernetesVersion, err := kubernetesVersion(clusterConfig)
	if err != nil {
		return nodeSpec{}, err
	}

	images := in.MetaConfig.Images.ConvertToMap()

	extensions, err := sysextExtensions(images, kubernetesVersion)
	if err != nil {
		return nodeSpec{}, err
	}

	registry, err := nodeRegistry(in.MetaConfig)
	if err != nil {
		return nodeSpec{}, err
	}

	pauseImage, err := sandboxImage(registry, images)
	if err != nil {
		return nodeSpec{}, err
	}

	osImageRef, err := osImageDigest(images)
	if err != nil {
		return nodeSpec{}, err
	}

	spec := nodeSpec{
		NodeName: in.NodeName,
		OSImage:  osImageRef,
		Storage: storage{
			DiskSelector: &diskSelector{Size: systemDiskSize},
			Mounts:       etcdMounts,
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
		Kubelet: nodeKubelet(in.MetaConfig, kubernetesVersion, maxPods(clusterConfig)),
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

	if in.Join != nil {
		spec.Kubelet.CACert = in.Join.CACert
		spec.Kubelet.BootstrapToken = in.Join.BootstrapToken
		spec.APIServerEndpoints = in.Join.APIServerEndpoints
		// Deckhouse is running by now, so the serving CSR gets approved. Left off,
		// this node would keep a self-signed serving certificate — no kubectl exec or
		// logs against it, for the life of the cluster.
		spec.Kubelet.ServerTLSBootstrap = nil
	}

	return spec, nil
}

// nodeKubelet is the kubelet section in the shape the node that starts the
// cluster needs it; the join branch of buildNodeSpec turns it into what a node
// joining a running cluster needs.
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

// podsPerNodeCIDRPrefix is the ladder bashible computes in
// candi/bashible/common-steps/all/064_configure_kubelet.sh.tpl — the scheduler
// believes it as capacity, so a master off the fleet's number skews placement.
var podsPerNodeCIDRPrefix = map[int]int{24: 120, 23: 250, 22: 500, 21: 1000}

// maxPods takes the ladder step of the cluster's per-node prefix. A prefix
// outside the ladder takes the nearest step, as the template's ge/le branches
// do; the top step is also the maximum the nodeConfig schema accepts.
func maxPods(clusterConfig map[string]any) int {
	prefix := defaultPodSubnetNodeCIDRPrefix
	if raw, ok := clusterConfig["podSubnetNodeCIDRPrefix"].(string); ok {
		if parsed, err := strconv.Atoi(raw); err == nil {
			prefix = parsed
		}
	}

	steps := slices.Sorted(maps.Keys(podsPerNodeCIDRPrefix))
	return podsPerNodeCIDRPrefix[min(max(prefix, steps[0]), steps[len(steps)-1])]
}

// kubernetesVersion is the cluster's Kubernetes minor version with "Automatic"
// already resolved to the installer default by ClusterConfigMap.
func kubernetesVersion(clusterConfig map[string]any) (string, error) {
	version, _ := clusterConfig["kubernetesVersion"].(string)
	if version == "" {
		return "", errors.New("kubernetesVersion is empty in the cluster configuration")
	}
	return version, nil
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

// etcdMounts gives a control-plane node the disk etcd lives on. The disk is
// described, not named — no /dev path exists before the machine does. No second
// disk matches nothing, which is supported: etcd shares the data partition.
var etcdMounts = []mount{{
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
