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
	"fmt"
	"net"
	"strings"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/manifests"
)

// nodeAddressPlaceholder stands for the first master's own address. This payload
// is built before the machine exists, so the address cannot be known here; the
// node substitutes its own when it writes the files. It is the placeholder
// bashible and control-plane-manager have always used on this path.
const nodeAddressPlaceholder = "$MY_IP"

// controlPlaneBundle is everything the node writes into /etc/kubernetes,
// rendered here rather than there.
type controlPlaneBundle struct {
	// Manifests are the four static pods, in the order they must be written:
	// kubelet starts a pod the moment its manifest appears, and an apiserver
	// started before its datastore only crash-loops.
	Manifests []renderedFile
	// ExtraFiles are the files the manifests reference by path instead of
	// carrying inline.
	ExtraFiles []renderedFile
}

// controlPlaneRenderParams are the cluster-wide settings the manifests are
// rendered from. Wider than the clusterParamsSpec that travels in the payload:
// four of these decide component command lines and nothing else, so they are
// consumed here rather than sent to a node that has nothing left to render.
type controlPlaneRenderParams struct {
	ClusterDomain     string
	ServiceSubnetCIDR string
	PodSubnetCIDR     string
	// PodSubnetNodeCIDRPrefix is the per-node prefix length, e.g. "24".
	PodSubnetNodeCIDRPrefix string
	// KubernetesVersion is the minor version, e.g. "1.34". "Automatic" is
	// resolved before it gets here.
	KubernetesVersion string
	// ClusterType is Cloud or Static. Without Cloud the controller manager
	// never gets --cloud-provider=external and never hands node lifecycle to
	// the cloud-controller-manager.
	ClusterType         string
	EncryptionAlgorithm string
	CertSANs            []string
}

// controlPlaneImages are the digests of the four static-pod images. The
// templates turn each into a reference as registry address + path + "@" +
// digest, the way they always have.
type controlPlaneImages struct {
	Etcd                  string
	KubeAPIServer         string
	KubeControllerManager string
	KubeScheduler         string
}

// manifestsInput is the render context, typed.
type manifestsInput struct {
	// NodeName is the etcd member name and the name the node registers under.
	NodeName string
	// NodeIP is the node's own address, or nodeAddressPlaceholder when the
	// machine does not exist yet — which is every bootstrap.
	NodeIP string
	// Cluster are the cluster-wide settings behind the component command lines.
	Cluster controlPlaneRenderParams
	// Registry is where the control-plane images are pulled from. The reference
	// is address + path + "@" + digest, so both halves are needed.
	Registry *registrySpec
	// Images are the digests of the four control-plane images.
	Images controlPlaneImages
}

// renderControlPlaneBundle renders what the first master's control plane is
// made of.
//
// It runs in the installer rather than on the node because only the installer
// knows the release it belongs to: the image digests, the template revision and
// the cluster settings all come from here. The node receives files and writes
// them, which leaves it with nothing to render and nothing to be a version of.
func renderControlPlaneBundle(ctx context.Context, in manifestsInput) (*controlPlaneBundle, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	data := in.data()
	node := manifests.NodeInput{NodeName: in.NodeName, NodeIP: in.NodeIP}

	rendered, err := manifests.Render(ctx, data, node)
	if err != nil {
		return nil, fmt.Errorf("render the control-plane manifests: %w", err)
	}
	extra, err := manifests.RenderExtraFiles(ctx, data, node)
	if err != nil {
		return nil, fmt.Errorf("render the control-plane extra files: %w", err)
	}

	return &controlPlaneBundle{
		Manifests:  filesOf(rendered),
		ExtraFiles: filesOf(extra),
	}, nil
}

func filesOf(bundle manifests.Bundle) []renderedFile {
	files := make([]renderedFile, 0, len(bundle))
	for _, artifact := range bundle {
		files = append(files, renderedFile{Name: artifact.Name, Content: string(artifact.Content)})
	}
	return files
}

// data turns the typed input into the map the templates index into. Three of
// its decisions look arbitrary and are not — each is a way the map can be wrong
// that the struct cannot.
func (in manifestsInput) data() map[string]any {
	imageKeySuffix := strings.ReplaceAll(in.Cluster.KubernetesVersion, ".", "")

	return map[string]any{
		// A constant, not an input, and that is the single most important line
		// here: it is what tells the templates there is no cluster to read from
		// yet.
		"runType":  manifests.RunTypeClusterBootstrap,
		"nodeName": in.NodeName,
		"nodeIP":   in.NodeIP,
		"registry": map[string]any{
			"address": in.Registry.Address,
			"path":    in.Registry.Path,
		},
		// The image keys carry the Kubernetes minor with the dot removed —
		// kubeApiserver134 — because the templates look them up as `printf
		// "kubeApiserver%s" (.clusterConfiguration.kubernetesVersion | replace
		// "." "")`.
		"images": map[string]any{
			"controlPlaneManager": map[string]any{
				"etcd":                                   in.Images.Etcd,
				"kubeApiserver" + imageKeySuffix:         in.Images.KubeAPIServer,
				"kubeControllerManager" + imageKeySuffix: in.Images.KubeControllerManager,
				"kubeScheduler" + imageKeySuffix:         in.Images.KubeScheduler,
			},
		},
		"clusterConfiguration": map[string]any{
			"kubernetesVersion":       in.Cluster.KubernetesVersion,
			"clusterDomain":           in.Cluster.ClusterDomain,
			"serviceSubnetCIDR":       in.Cluster.ServiceSubnetCIDR,
			"podSubnetCIDR":           in.Cluster.PodSubnetCIDR,
			"podSubnetNodeCIDRPrefix": in.Cluster.PodSubnetNodeCIDRPrefix,
			// Always a key, even when empty: kube-controller-manager.yaml.tpl
			// compares it with `eq`, and `eq` on a missing key is a template
			// error rather than a false.
			"clusterType": in.Cluster.ClusterType,
		},
		// Empty on purpose. The first master runs the manifests these two
		// produce nothing for; control-plane-manager renders them again with
		// the operator's settings as soon as it is installed.
		"apiserver": map[string]any{},
		"settings":  map[string]any{},
	}
}

func (in manifestsInput) validate() error {
	if in.Registry == nil {
		return fmt.Errorf("registry is nil")
	}

	required := []struct {
		field string
		value string
	}{
		{"nodeName", in.NodeName},
		{"nodeIP", in.NodeIP},
		{"cluster.kubernetesVersion", in.Cluster.KubernetesVersion},
		{"cluster.clusterDomain", in.Cluster.ClusterDomain},
		{"cluster.serviceSubnetCIDR", in.Cluster.ServiceSubnetCIDR},
		{"cluster.podSubnetCIDR", in.Cluster.PodSubnetCIDR},
		{"cluster.podSubnetNodeCIDRPrefix", in.Cluster.PodSubnetNodeCIDRPrefix},
		{"registry.address", in.Registry.Address},
		{"images.etcd", in.Images.Etcd},
		{"images.kubeApiserver", in.Images.KubeAPIServer},
		{"images.kubeControllerManager", in.Images.KubeControllerManager},
		{"images.kubeScheduler", in.Images.KubeScheduler},
	}
	for _, r := range required {
		if strings.TrimSpace(r.value) == "" {
			return fmt.Errorf("%s must not be empty", r.field)
		}
	}

	// Either a real address or the placeholder. Anything else reaches the node
	// as an --advertise-address it cannot bind and an etcd URL nobody answers.
	if in.NodeIP != nodeAddressPlaceholder && net.ParseIP(in.NodeIP) == nil {
		return fmt.Errorf("nodeIP %q is neither an IP address nor %s", in.NodeIP, nodeAddressPlaceholder)
	}
	return nil
}
