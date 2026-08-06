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
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

// controlPlaneDigestsKey is the images_digests.json module the control-plane
// component images are built in.
const controlPlaneDigestsKey = "controlPlaneManager"

func (in MasterPayloadInput) validate() error {
	switch {
	case in.NodeName == "":
		return errors.New("node name is empty")
	case in.MetaConfig == nil:
		return errors.New("meta config is nil")
	case in.StateCache == nil:
		return errors.New("state cache is nil")
	case in.CandiDir == "":
		return errors.New("candi dir is empty")
	}
	return nil
}

// buildControlPlaneConfig assembles what the first master brings its own control
// plane up from: the settings it still decides for itself, and the manifests it
// only writes.
//
// The only key in the result belongs to the handoff endpoint dhctl collects the
// admin kubeconfig through. The cluster PKI is generated on the node and never
// comes near this document.
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

	bundle, err := renderControlPlaneBundle(ctx, manifestsInput{
		NodeName: in.NodeName,
		NodeIP:   nodeAddressPlaceholder,
		Cluster:  cluster,
		Registry: registry,
		Images:   images,
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
			Bootstrap: true,
			Cluster: clusterParamsSpec{
				ClusterDomain:       cluster.ClusterDomain,
				ServiceSubnetCIDR:   cluster.ServiceSubnetCIDR,
				EncryptionAlgorithm: cluster.EncryptionAlgorithm,
				CertSANs:            cluster.CertSANs,
			},
			Manifests:  bundle.Manifests,
			ExtraFiles: bundle.ExtraFiles,
			Handoff:    handoffPayload(*handoff),
		},
	}, nil
}

// clusterParams reads the cluster-wide inputs. Every one of them ends up in a
// certificate SAN or a component flag, so an empty one is not passed through:
// it renders as an empty flag and the component dies on its own command line
// with a message that says nothing about where it came from.
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
		ClusterType:         metaConfig.ClusterType,
		EncryptionAlgorithm: encryption,
		CertSANs:            certSANs(metaConfig),
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

// ResolveControlPlaneImages picks the digests of the four static-pod images out
// of the digest map baked into the installer image. The templates turn them into
// references; a preflight check calls this on its own to fail a cluster whose
// Kubernetes version this installer ships no control plane for.
//
// Pure; the context is here for the package's uniform exported signature.
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
