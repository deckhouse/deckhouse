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
	"path/filepath"
	"sort"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

// nodeAddressPlaceholder stands for the first master's own address. This payload
// is built before the machine exists, so the address cannot be known here; the
// node substitutes its own when it writes the files. It is the placeholder
// bashible and control-plane-manager have always used on this path.
const nodeAddressPlaceholder = "$MY_IP"

// controlPlaneTemplatesDir is where the control-plane templates live, relative
// to CandiDir. The same directory the classic bootstrap renders from, through
// the same engine — which is what makes a master brought up this way run the
// bytes a master brought up classically runs.
const controlPlaneTemplatesDir = "control-plane"

// runTypeClusterBootstrap is the run that brings the first control plane up,
// before there is a cluster to read anything from. The templates gate several
// flags on it.
const runTypeClusterBootstrap = "ClusterBootstrap"

// etcdManifest is written first: kubelet starts a pod the moment its manifest
// appears, and an apiserver started before its datastore only crash-loops.
const etcdManifest = "etcd.yaml"

// authenticationConfig is the one file kube-apiserver demands on every run,
// bootstrap included — the manifest passes --authentication-config
// unconditionally, and an apiserver that cannot read the file exits before it
// opens a port.
const authenticationConfig = "authentication-config.yaml"

// bootstrapAuthenticationConfig is what bashible writes into that file before it
// copies the kube-apiserver manifest into place, quoted byte for byte from the
// heredoc in candi/bashible/common-steps/cluster-bootstrap/072_install_control_plane.sh.tpl.
//
// A constant rather than a template because at bootstrap it has no variables:
// everything the module's helm define branches on — an OIDC issuer, a CA — comes
// from a ModuleConfig that does not exist yet. It is the minimum kube-apiserver
// needs to answer its own probes; control-plane-manager rewrites the file from
// its own render as soon as Deckhouse is installed.
const bootstrapAuthenticationConfig = `apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthenticationConfiguration
anonymous:
  enabled: true
  conditions:
  - path: /livez
  - path: /readyz
  - path: /healthz
`

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
	// CandiDir is the installer's own candi directory. The templates come from
	// there rather than from the provider image, which ships no control-plane
	// templates at all.
	CandiDir string
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

	dir := filepath.Join(in.CandiDir, controlPlaneTemplatesDir)
	rendered, err := template.RenderTemplatesDir(ctx, dir, in.data(), nil)
	if err != nil {
		return nil, fmt.Errorf("render the control-plane manifests from %s: %w", dir, err)
	}
	// A missing directory is not an error to RenderTemplatesDir — it logs and
	// returns nothing. Here it would mean a node that waits for an apiserver
	// nobody is going to start, so it has to be one.
	if len(rendered) == 0 {
		return nil, fmt.Errorf("no control-plane templates in %s", dir)
	}

	manifests := make([]renderedFile, 0, len(rendered))
	for _, artifact := range rendered {
		manifests = append(manifests, renderedFile{
			Name:    artifact.FileName,
			Content: artifact.Content.String(),
		})
	}
	sortEtcdFirst(manifests)

	return &controlPlaneBundle{
		Manifests: manifests,
		ExtraFiles: []renderedFile{
			{Name: authenticationConfig, Content: bootstrapAuthenticationConfig},
		},
	}, nil
}

// sortEtcdFirst puts etcd at the head and the rest in name order, so the same
// inputs always produce the same sequence of writes on the node.
func sortEtcdFirst(files []renderedFile) {
	sort.Slice(files, func(i, j int) bool {
		switch {
		case files[i].Name == etcdManifest:
			return true
		case files[j].Name == etcdManifest:
			return false
		default:
			return files[i].Name < files[j].Name
		}
	})
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
		"runType":  runTypeClusterBootstrap,
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
		// Empty on purpose, and load-bearing twice over: the first master runs
		// the manifests these two produce nothing for, and it is their emptiness
		// that makes bootstrapAuthenticationConfig a constant rather than a
		// render. control-plane-manager fills both in from the operator's
		// settings as soon as it is installed.
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
		{"candiDir", in.CandiDir},
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
