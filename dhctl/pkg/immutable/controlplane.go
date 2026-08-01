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
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/pki"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/module/controlplane"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

// CACacheKey names the CA bundle in the dhctl state cache. A second bootstrap
// attempt must hand the node the same CA: a fresh one would invalidate every
// certificate and kubeconfig the node already issued from the first payload.
const CACacheKey = "immutable-control-plane-ca"

// caFiles are the eight artifacts the node needs to run its own PKI. The paths
// are relative to /etc/kubernetes/pki and match the layout pki.CreatePKIBundle
// writes.
var caFiles = []string{
	"ca.crt", "ca.key",
	"front-proxy-ca.crt", "front-proxy-ca.key",
	"etcd/ca.crt", "etcd/ca.key",
	"sa.key", "sa.pub",
}

// authenticationConfig lets the apiserver answer its own health probes before
// any RBAC exists. Kept byte for byte in sync with the heredoc in
// candi/bashible/common-steps/cluster-bootstrap/072_install_control_plane.sh.tpl.
const authenticationConfig = `apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthenticationConfiguration
anonymous:
  enabled: true
  conditions:
  - path: /livez
  - path: /readyz
  - path: /healthz
`

const authenticationConfigFile = "authentication-config.yaml"

// extraFilesDir is where the control-plane manifests expect the contents of
// ControlPlaneSpec.ExtraFiles to be on the node.
const extraFilesDir = "/etc/kubernetes/deckhouse/extra-files"

// ControlPlaneInput is everything BuildControlPlaneConfig needs.
type ControlPlaneInput struct {
	// NodeName is the name the first master registers under.
	NodeName string
	// MetaConfig is the parsed cluster configuration.
	MetaConfig *config.MetaConfig
	// GlobalOpts locates candi/control-plane and the OpenAPI schemas.
	GlobalOpts *options.GlobalOptions
	// StateCache carries the CA bundle between bootstrap attempts.
	StateCache state.Cache
}

func (in ControlPlaneInput) validate() error {
	switch {
	case in.NodeName == "":
		return fmt.Errorf("node name is empty")
	case in.MetaConfig == nil:
		return fmt.Errorf("meta config is nil")
	case in.GlobalOpts == nil:
		return fmt.Errorf("global options are nil")
	case in.StateCache == nil:
		return fmt.Errorf("state cache is nil")
	}
	return nil
}

// BuildControlPlaneConfig generates the CA bundle and renders the control-plane
// manifests for the first master. The node issues its own leaf certificates and
// kubeconfigs from this CA once it knows its address.
func BuildControlPlaneConfig(ctx context.Context, in ControlPlaneInput) (*ControlPlaneConfig, error) {
	if err := in.validate(); err != nil {
		return nil, fmt.Errorf("build control-plane config: %w", err)
	}

	clusterDomain := in.MetaConfig.ClusterDomain
	if clusterDomain == "" {
		return nil, fmt.Errorf("clusterDomain is empty in the cluster configuration")
	}

	serviceSubnetCIDR, err := clusterConfigString(in.MetaConfig, "serviceSubnetCIDR")
	if err != nil {
		return nil, err
	}
	if serviceSubnetCIDR == "" {
		return nil, fmt.Errorf("serviceSubnetCIDR is empty in the cluster configuration")
	}

	encryptionAlgorithm, err := encryptionAlgorithm(in.MetaConfig)
	if err != nil {
		return nil, err
	}

	_, controlPlaneDisk, err := MasterDisks(in.MetaConfig)
	if err != nil {
		return nil, err
	}

	ca, err := caBundle(ctx, in, clusterDomain, serviceSubnetCIDR, encryptionAlgorithm)
	if err != nil {
		return nil, err
	}

	manifests, err := renderControlPlaneManifests(ctx, in.MetaConfig, in.GlobalOpts)
	if err != nil {
		return nil, err
	}

	extraFiles := map[string]string{authenticationConfigFile: authenticationConfig}

	if err := checkExtraFiles(manifests, extraFiles); err != nil {
		return nil, err
	}

	return &ControlPlaneConfig{
		APIVersion: PayloadAPIVersion,
		Kind:       ControlPlaneConfigKind,
		Metadata:   ObjectMeta{Name: in.NodeName},
		Spec: ControlPlaneSpec{
			Bootstrap: true,
			Disk:      controlPlaneDisk,
			CA:        ca,
			Params: ControlPlaneParams{
				ClusterDomain:       clusterDomain,
				ServiceSubnetCIDR:   serviceSubnetCIDR,
				EncryptionAlgorithm: encryptionAlgorithm,
				CertSANs:            certSANs(in.MetaConfig),
			},
			ExtraFiles: extraFiles,
			Manifests:  manifests,
		},
	}, nil
}

// extraFileRef finds every /etc/kubernetes/deckhouse/extra-files/<name> the
// rendered manifests point a flag at.
var extraFileRef = regexp.MustCompile(regexp.QuoteMeta(extraFilesDir) + `/([A-Za-z0-9._-]+)`)

// checkExtraFiles refuses a payload whose manifests start a component with a
// file nobody puts on the node.
//
// On the classic path those files are written by the module preparators
// bashible runs before the manifests appear; none of them runs here, so
// anything beyond the hardcoded set above is a component that crash-loops on a
// missing file with no way to tell why from the outside. Whichever cluster
// setting turned the flag on has to grow support here first.
func checkExtraFiles(manifests, extraFiles map[string]string) error {
	missing := make(map[string][]string)

	names := make([]string, 0, len(manifests))
	for name := range manifests {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, manifest := range names {
		for _, match := range extraFileRef.FindAllStringSubmatch(manifests[manifest], -1) {
			file := match[1]
			if _, ok := extraFiles[file]; ok {
				continue
			}
			missing[file] = append(missing[file], manifest)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	files := make([]string, 0, len(missing))
	for file, manifests := range missing {
		files = append(files, fmt.Sprintf("%s (referenced by %s)", file, strings.Join(manifests, ", ")))
	}
	sort.Strings(files)

	return fmt.Errorf(
		"the rendered control-plane manifests need extra files the immutable bootstrap does not produce: %s; "+
			"the cluster configuration enables a control-plane feature that is not supported on an immutable master yet",
		strings.Join(files, "; "),
	)
}

// certSANs are the extra names the apiserver certificate must cover. The node
// issues that certificate itself, so it needs the same list
// control-plane-manager later publishes under the "cert-sans" key of its config
// secret — without them anything reaching the cluster through a load balancer
// or a floating IP fails the hostname check until control-plane-manager
// reissues the certificate.
func certSANs(metaConfig *config.MetaConfig) []string {
	mc := metaConfig.FindModuleConfig("control-plane-manager")
	if mc == nil {
		return nil
	}

	apiserver, ok := mc.Spec.Settings["apiserver"].(map[string]any)
	if !ok {
		return nil
	}

	raw, ok := apiserver["certSANs"].([]any)
	if !ok {
		return nil
	}

	sans := make([]string, 0, len(raw))
	for _, value := range raw {
		if san, ok := value.(string); ok && san != "" {
			sans = append(sans, san)
		}
	}
	if len(sans) == 0 {
		return nil
	}
	return sans
}

// LoadCABundle returns the CA bundle a previous BuildControlPlaneConfig stored,
// or nil when the cache holds none. dhctl reads it back to mint its own client
// certificate for the API server the node brings up.
func LoadCABundle(ctx context.Context, cache state.Cache) (map[string]string, error) {
	inCache, err := cache.InCache(ctx, CACacheKey)
	if err != nil {
		return nil, fmt.Errorf("look up %s in the state cache: %w", CACacheKey, err)
	}
	if !inCache {
		return nil, nil
	}

	var bundle map[string]string
	if err := cache.LoadStruct(ctx, CACacheKey, &bundle); err != nil {
		return nil, fmt.Errorf("load %s from the state cache: %w", CACacheKey, err)
	}
	return bundle, nil
}

// caBundle reuses the cached CA or generates a fresh one and caches it.
func caBundle(ctx context.Context, in ControlPlaneInput, clusterDomain, serviceSubnetCIDR, encryptionAlgorithm string) (map[string]string, error) {
	cached, err := LoadCABundle(ctx, in.StateCache)
	if err != nil {
		return nil, err
	}
	if len(cached) > 0 {
		dhlog.FromContext(ctx).InfoContext(ctx, "Reusing the control-plane CA bundle from the state cache")
		return cached, nil
	}

	bundle, err := generateCABundle(in.NodeName, clusterDomain, serviceSubnetCIDR, encryptionAlgorithm)
	if err != nil {
		return nil, err
	}

	if err := in.StateCache.SaveStruct(ctx, CACacheKey, bundle); err != nil {
		return nil, fmt.Errorf("save the control-plane CA bundle to the state cache: %w", err)
	}

	return bundle, nil
}

// generateCABundle creates the three CAs and the ServiceAccount key pair in a
// temporary directory and reads them back.
//
// CreatePKIBundle also issues the leaf certificates; they are thrown away. The
// node re-issues them with its own address in the SAN list once it has one,
// which is also why the advertise address here is 127.0.0.1: at payload time
// the VM does not exist, and that address only ever reaches the discarded
// leaves.
func generateCABundle(nodeName, clusterDomain, serviceSubnetCIDR, encryptionAlgorithm string) (map[string]string, error) {
	pkiDir, err := os.MkdirTemp("", "dhctl-immutable-pki-")
	if err != nil {
		return nil, fmt.Errorf("create a temporary PKI directory: %w", err)
	}
	defer os.RemoveAll(pkiDir)

	if _, err := pki.CreatePKIBundle(
		nodeName,
		clusterDomain,
		net.ParseIP(constants.DefaultControlPlaneIP),
		serviceSubnetCIDR,
		pki.WithPKIDir(pkiDir),
		pki.WithEncryptionAlgorithmType(constants.EncryptionAlgorithmType(encryptionAlgorithm)),
	); err != nil {
		return nil, fmt.Errorf("create the PKI bundle: %w", err)
	}

	bundle := make(map[string]string, len(caFiles))
	for _, name := range caFiles {
		content, err := os.ReadFile(filepath.Join(pkiDir, name))
		if err != nil {
			return nil, fmt.Errorf("read %s from the generated PKI bundle: %w", name, err)
		}
		bundle[name] = string(content)
	}

	return bundle, nil
}

// renderControlPlaneManifests renders candi/control-plane/*.yaml.tpl into
// memory. The node lays the result down under /etc/kubernetes/manifests.
func renderControlPlaneManifests(ctx context.Context, metaConfig *config.MetaConfig, globalOpts *options.GlobalOptions) (map[string]string, error) {
	extractor := controlplane.NewSettingsExtractor(
		metaConfig,
		config.NewSchemaStore(globalOpts),
		config.GetEdition(),
		dhlog.FromContext(ctx),
	)

	// An empty node IP keeps the "$MY_IP"/"$MY_NODENAME" placeholders in the
	// rendered manifests. The VM does not exist yet; the node expands them from
	// its own address and hostname when it writes the manifests out.
	templateConfig, err := extractor.TemplateConfigForBootstrap("")
	if err != nil {
		return nil, fmt.Errorf("build the control-plane template context: %w", err)
	}

	dir := filepath.Join(globalOpts.CandiDir, "control-plane")
	rendered, err := template.RenderTemplatesDir(ctx, dir, templateConfig.ToMap(), nil)
	if err != nil {
		return nil, fmt.Errorf("render control-plane manifests from %s: %w", dir, err)
	}
	if len(rendered) == 0 {
		return nil, fmt.Errorf("no control-plane manifests were rendered from %s", dir)
	}

	manifests := make(map[string]string, len(rendered))
	names := make([]string, 0, len(rendered))
	for _, tpl := range rendered {
		manifests[tpl.FileName] = tpl.Content.String()
		names = append(names, tpl.FileName)
	}
	sort.Strings(names)
	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Rendered control-plane manifests: %v", names))

	return manifests, nil
}

// encryptionAlgorithm reads the algorithm the cluster pins for its keys. An
// empty result means "use the library default" and is passed through as such.
func encryptionAlgorithm(metaConfig *config.MetaConfig) (string, error) {
	mc := metaConfig.FindModuleConfig("control-plane-manager")
	if mc != nil {
		if value, ok := mc.Spec.Settings["encryptionAlgorithm"].(string); ok && value != "" {
			return value, nil
		}
	}

	return clusterConfigString(metaConfig, "encryptionAlgorithm")
}
