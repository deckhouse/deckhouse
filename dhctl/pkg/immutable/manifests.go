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
	"path/filepath"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

// nodeAddressPlaceholder stands for the first master's own address, which this
// payload cannot know: it is built before the machine exists. The node puts its
// own in — the placeholder bashible has always used on this path.
const nodeAddressPlaceholder = "$MY_IP"

// controlPlaneTemplatesDir is where the templates live under CandiDir. The same
// directory the classic bootstrap renders from, through the same engine, so both
// kinds of master run the same bytes.
const controlPlaneTemplatesDir = "control-plane"

// runTypeClusterBootstrap is the run that brings the first control plane up,
// before there is a cluster to read anything from.
const runTypeClusterBootstrap = "ClusterBootstrap"

// etcdManifest is written first: kubelet starts a pod the moment its manifest
// appears, and an apiserver started before its datastore only crash-loops.
const etcdManifest = "etcd.yaml"

// authenticationConfig is the one file kube-apiserver demands on every run: the
// manifest passes --authentication-config unconditionally, and an apiserver that
// cannot read it exits before opening a port.
const authenticationConfig = "authentication-config.yaml"

// bootstrapAuthenticationConfig is what bashible writes into that file, quoted
// byte for byte from
// candi/bashible/common-steps/cluster-bootstrap/072_install_control_plane.sh.tpl.
const bootstrapAuthenticationConfig = `apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthenticationConfiguration
anonymous:
  enabled: true
  conditions:
  - path: /livez
  - path: /readyz
  - path: /healthz
`

// controlPlaneRenderParams are the settings the manifests are rendered from.
// Wider than the clusterParamsSpec that travels in the payload: the four fields
// below it decide component command lines and are consumed here.
type controlPlaneRenderParams struct {
	clusterParamsSpec
	PodSubnetCIDR string
	// PodSubnetNodeCIDRPrefix is the per-node prefix length, e.g. "24".
	PodSubnetNodeCIDRPrefix string
	// KubernetesVersion is the minor version, e.g. "1.34". "Automatic" is
	// resolved before it gets here.
	KubernetesVersion string
	// ClusterType is Cloud or Static. Without Cloud the controller manager
	// never gets --cloud-provider=external and never hands node lifecycle to
	// the cloud-controller-manager.
	ClusterType string
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
	// Settings are the operator's control-plane-manager settings. Only
	// resourcesRequests is read at bootstrap, and only by these four templates;
	// empty leaves them on the defaults written into the templates.
	Settings map[string]any
	// CandiDir is the installer's own candi directory. The templates come from
	// there rather than from the provider image, which ships no control-plane
	// templates at all.
	CandiDir string
}

// bootstrapExtraFiles are the files the manifests reference by path instead of
// carrying inline; they depend on nothing the render reads.
var bootstrapExtraFiles = []renderedFile{{Name: authenticationConfig, Content: bootstrapAuthenticationConfig}}

// renderControlPlaneBundle renders the static pods the first master's control
// plane is made of. Here rather than on the node: only the installer knows its
// own release, and the digests, templates and settings all come from it.
func renderControlPlaneBundle(ctx context.Context, in manifestsInput) ([]renderedFile, error) {
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

	// RenderTemplatesDir reads the directory with os.ReadDir, which sorts by
	// name, so the node is written the same sequence on every render.
	manifests := make([]renderedFile, 0, len(rendered))
	for _, artifact := range rendered {
		manifests = append(manifests, renderedFile{
			Name:    artifact.FileName,
			Content: artifact.Content.String(),
		})
	}

	return manifests, nil
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
		// The keys carry the Kubernetes minor with the dot removed —
		// kubeApiserver134 — because the templates build the name with
		// `printf "kubeApiserver%s" (… | replace "." "")`.
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
		// Empty on purpose, and load-bearing: its emptiness makes
		// bootstrapAuthenticationConfig a constant. The only key the templates
		// would read is the signature mode, which a preflight refuses outright.
		"apiserver": map[string]any{},
		"settings":  in.Settings,
	}
}
