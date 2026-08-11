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
	"strings"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/module/controlplane"
)

// controlPlaneDigestsKey is the images_digests.json module the control-plane
// component images are built in.
const controlPlaneDigestsKey = "controlPlaneManager"

// validate covers what buildNodeConfig, which runs first, does not: it already
// rejects an empty node name and a nil meta config.
func (in MasterPayloadInput) validate() error {
	switch {
	case in.StateCache == nil:
		return errors.New("state cache is nil")
	case in.CandiDir == "":
		return errors.New("candi dir is empty")
	case in.GlobalOptions == nil:
		return errors.New("global options are nil")
	}
	return nil
}

// controlPlaneSettings are the operator's control-plane-manager settings, in the
// shape the templates read them in. Read through the same extractor the classic
// bootstrap uses so both masters honour settings like resourcesRequests alike.
func controlPlaneSettings(ctx context.Context, in MasterPayloadInput) (map[string]any, error) {
	extractor := controlplane.NewSettingsExtractor(
		in.MetaConfig,
		config.NewSchemaStore(in.GlobalOptions),
		config.GetEdition(),
		dhlog.FromContext(ctx),
	)

	cfg, err := extractor.TemplateConfigForBootstrap(nodeAddressPlaceholder)
	if err != nil {
		return nil, fmt.Errorf("read control-plane-manager settings: %w", err)
	}
	return cfg.Settings, nil
}

// buildControlPlaneConfig assembles what the first master brings its own
// control plane up from. The only key in the result belongs to the handoff
// endpoint; the cluster PKI is generated on the node and never enters it.
func buildControlPlaneConfig(ctx context.Context, in MasterPayloadInput) (*controlPlaneConfig, error) {
	if err := in.validate(); err != nil {
		return nil, fmt.Errorf("build control-plane config: %w", err)
	}

	cluster, err := clusterParams(in.MetaConfig)
	if err != nil {
		return nil, err
	}

	images, err := ResolveControlPlaneImages(ctx, in.MetaConfig)
	if err != nil {
		return nil, err
	}

	registry, err := nodeRegistry(in.MetaConfig)
	if err != nil {
		return nil, err
	}

	settings, err := controlPlaneSettings(ctx, in)
	if err != nil {
		return nil, err
	}

	manifests, extraFiles, err := renderControlPlaneBundle(ctx, manifestsInput{
		NodeName: in.NodeName,
		NodeIP:   nodeAddressPlaceholder,
		Cluster:  cluster,
		Registry: registry,
		Images:   images,
		Settings: settings,
		CandiDir: in.CandiDir,
	})
	if err != nil {
		return nil, err
	}

	handoff, err := HandoffMaterialFor(ctx, in.StateCache, in.NodeName)
	if err != nil {
		return nil, err
	}

	return &controlPlaneConfig{
		APIVersion: payloadAPIVersion,
		Kind:       controlPlaneConfigKind,
		Metadata:   objectMeta{Name: in.NodeName},
		Spec: controlPlaneSpec{
			Bootstrap:  true,
			Cluster:    cluster.clusterParamsSpec,
			Manifests:  manifests,
			ExtraFiles: extraFiles,
			Handoff:    handoffPayload(*handoff),
		},
	}, nil
}

// clusterParams reads the cluster-wide inputs. Every one ends up in a
// certificate SAN or a component flag, so an empty one is rejected here instead
// of rendering an empty flag the component dies on with an opaque message.
func clusterParams(metaConfig *config.MetaConfig) (controlPlaneRenderParams, error) {
	// ClusterConfigMap resolves an "Automatic" kubernetesVersion to the version
	// this installer defaults to. Rendering "Automatic" into the feature gates
	// of every component is what the raw value would do.
	clusterConfig, err := metaConfig.ClusterConfigMap()
	if err != nil {
		return controlPlaneRenderParams{}, fmt.Errorf("read the cluster configuration: %w", err)
	}

	encryption, err := encryptionAlgorithm(metaConfig)
	if err != nil {
		return controlPlaneRenderParams{}, err
	}

	params := controlPlaneRenderParams{
		clusterParamsSpec: clusterParamsSpec{
			EncryptionAlgorithm: encryption,
			CertSANs:            certSANs(metaConfig),
		},
		ClusterType: metaConfig.ClusterType,
	}

	required := []struct {
		key    string
		target *string
	}{
		{"clusterDomain", &params.ClusterDomain},
		{"serviceSubnetCIDR", &params.ServiceSubnetCIDR},
		{"podSubnetCIDR", &params.PodSubnetCIDR},
		{"podSubnetNodeCIDRPrefix", &params.PodSubnetNodeCIDRPrefix},
		{"kubernetesVersion", &params.KubernetesVersion},
	}
	for _, field := range required {
		value, _ := clusterConfig[field.key].(string)
		if value == "" {
			return controlPlaneRenderParams{}, fmt.Errorf("%s is empty in the cluster configuration", field.key)
		}
		*field.target = value
	}

	if params.ClusterType == "" {
		return controlPlaneRenderParams{}, errors.New("clusterType is empty in the cluster configuration")
	}

	return params, nil
}

// ResolveControlPlaneImages picks the four static-pod image digests out of the
// map baked into the installer image; a preflight check calls it to fail early
// on an unsupported Kubernetes version. Pure; the context is for uniformity.
func ResolveControlPlaneImages(_ context.Context, metaConfig *config.MetaConfig) (controlPlaneImages, error) {
	version, err := kubernetesVersion(metaConfig)
	if err != nil {
		return controlPlaneImages{}, err
	}

	digests, err := digestGroup(metaConfig.Images.ConvertToMap(), controlPlaneDigestsKey)
	if err != nil {
		return controlPlaneImages{}, err
	}

	// The image names in images_digests.json are produced by the sprig
	// camelcase function, which strips the separators and the dots of the minor
	// version: kube-apiserver 1.34 becomes kubeApiserver134.
	minor := strings.ReplaceAll(version, ".", "")

	var images controlPlaneImages
	for _, image := range []struct {
		name   string
		target *string
	}{
		{"etcd", &images.Etcd},
		{"kubeApiserver" + minor, &images.KubeAPIServer},
		{"kubeControllerManager" + minor, &images.KubeControllerManager},
		{"kubeScheduler" + minor, &images.KubeScheduler},
	} {
		digest := digests[image.name]
		if digest == "" {
			return controlPlaneImages{}, fmt.Errorf(
				"the installer image carries no %q.%q image digest: it does not ship a control plane for Kubernetes %s",
				controlPlaneDigestsKey, image.name, version,
			)
		}
		*image.target = digest
	}

	return images, nil
}

// certSANs are the extra names the apiserver certificate must cover: the same
// list control-plane-manager publishes under "cert-sans". Without them access
// via a load balancer or floating IP fails the hostname check until reissue.
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

// encryptionAlgorithm reads the algorithm the cluster pins for its keys. An
// empty result means "use the library default" and is passed through as such.
func encryptionAlgorithm(metaConfig *config.MetaConfig) (string, error) {
	mc := metaConfig.FindModuleConfig("control-plane-manager")
	if mc != nil {
		if value, ok := mc.Spec.Settings["encryptionAlgorithm"].(string); ok && value != "" {
			return value, nil
		}
	}

	raw := metaConfig.ClusterConfig["encryptionAlgorithm"]
	if len(raw) == 0 {
		return "", nil
	}

	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("parse encryptionAlgorithm from the cluster configuration: %w", err)
	}
	return value, nil
}
