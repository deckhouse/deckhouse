/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cloudprovider

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/node-controller/internal/clusterprefix"
	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/machinetemplate"
)

const (
	ProviderTemplateSecretNamespace  = common.KubeSystemNamespace
	CAPIMachineTemplateKey           = "template.yaml"
	CAPIClusterTemplateKey           = "cluster.yaml"
	CAPICredentialsTemplateKey       = "credentials.yaml"
	CAPIClusterCredentialsSecretName = "capi-user-credentials"

	engineCAPI = "capi"
	engineMCM  = "mcm"

	legacyMachineTemplateKey = "machine-template.yaml"
	legacyCAPIChecksumKey    = "instance-class.checksum"
	machineClassTemplateKey  = "machine-class.yaml"
	machineClassChecksumKey  = "machine-class.checksum"
	machineClassConfigKey    = "config-for-machine-controller-manager.yaml"

	clusterUUIDConfigMapName = common.ClusterUUIDConfigMapName
	clusterUUIDConfigMapKey  = common.ClusterUUIDConfigMapKey
)

// Source is the common reader used by both CAPI controllers. Template-specific methods only
// parse the files needed by their caller, so an invalid cluster template cannot stop the machine
// controller and vice versa.
type Source struct {
	Reader client.Reader
}

// Provider is the common, validated input shared by provider templates.
type Provider struct {
	Registration Registration
	Cluster      machinetemplate.ClusterFacts
	Prefix       string
	LegacyValues map[string]any
}

func (p Provider) RenderData() RenderData {
	return RenderData{
		Provider: p.Registration.CloudVariables,
		Cluster:  p.Cluster,
		Prefix:   p.Prefix,
	}
}

type CAPIMachineInputs struct {
	Template              *MachineTemplate
	LegacyMachineTemplate []byte
	LegacyChecksum        []byte
}

type CAPIClusterInputs struct {
	Cluster     *ClusterTemplate
	Credentials *CredentialsTemplate
}

type MCMInputs struct {
	MachineClass []byte
	Checksum     []byte
	Config       []byte
}

// Load reads the cluster facts a provider template renders against. The registration is resolved
// by the caller — every NodeGroup picks its own through the catalog — so this never reads one.
func (s Source) Load(ctx context.Context, registration Registration) (Provider, error) {
	configuration, err := common.ReadClusterConfiguration(ctx, s.Reader)
	if err != nil {
		return Provider{}, err
	}
	return s.LoadWithClusterConfiguration(ctx, registration, configuration)
}

// LoadWithClusterConfiguration is Load for a caller that already read the cluster configuration
// for its own resources.
func (s Source) LoadWithClusterConfiguration(
	ctx context.Context,
	registration Registration,
	configuration common.ClusterConfiguration,
) (Provider, error) {
	if err := registration.ValidateCore(); err != nil {
		return Provider{}, err
	}
	if configuration.PodSubnetCIDR == "" {
		return Provider{}, errors.New("cluster configuration has no podSubnetCIDR")
	}

	clusterUUID, err := s.readClusterUUID(ctx)
	if err != nil {
		return Provider{}, err
	}
	prefix, err := clusterprefix.ResolveWithClusterConfiguration(ctx, s.Reader, configuration)
	if err != nil {
		return Provider{}, fmt.Errorf("resolve cluster prefix: %w", err)
	}

	return Provider{
		Registration: registration,
		Cluster: machinetemplate.ClusterFacts{
			Name:      registration.CAPIClusterName,
			Namespace: capiNamespace,
			UUID:      clusterUUID,
			PodSubnet: configuration.PodSubnetCIDR,
		},
		Prefix:       prefix,
		LegacyValues: registration.Data,
	}, nil
}

func (s Source) LoadCAPIMachineInputs(ctx context.Context, provider Provider) (CAPIMachineInputs, error) {
	registration := provider.Registration
	if err := registration.ValidateCAPI(); err != nil {
		return CAPIMachineInputs{}, err
	}
	data, err := s.readTemplateSecret(ctx, registration.Type, engineCAPI)
	if err != nil {
		return CAPIMachineInputs{}, err
	}

	inputs := CAPIMachineInputs{
		LegacyMachineTemplate: data[legacyMachineTemplateKey],
		LegacyChecksum:        data[legacyCAPIChecksumKey],
	}
	if raw, ok := data[CAPIMachineTemplateKey]; ok {
		contract, err := machinetemplate.ParseContract(raw)
		if err != nil {
			return CAPIMachineInputs{}, fmt.Errorf("cloud provider %s %s: %w", registration.Type, CAPIMachineTemplateKey, err)
		}
		inputs.Template = &MachineTemplate{Contract: contract}
		return inputs, nil
	}
	if len(inputs.LegacyMachineTemplate) == 0 || len(inputs.LegacyChecksum) == 0 {
		return CAPIMachineInputs{}, fmt.Errorf("cloud provider %s publishes neither %s nor the complete legacy CAPI template", registration.Type, CAPIMachineTemplateKey)
	}
	return inputs, nil
}

func (s Source) LoadCAPIClusterInputs(ctx context.Context, provider Provider) (CAPIClusterInputs, error) {
	registration := provider.Registration
	if err := registration.ValidateCAPI(); err != nil {
		return CAPIClusterInputs{}, err
	}
	data, err := s.readTemplateSecret(ctx, registration.Type, engineCAPI)
	if err != nil {
		return CAPIClusterInputs{}, err
	}
	raw, ok := data[CAPIClusterTemplateKey]
	if !ok {
		return CAPIClusterInputs{}, fmt.Errorf("cloud provider %s publishes no %s", registration.Type, CAPIClusterTemplateKey)
	}
	clusterTemplate, err := newClusterTemplate(raw, registration)
	if err != nil {
		return CAPIClusterInputs{}, fmt.Errorf("cloud provider %s: %w", registration.Type, err)
	}

	inputs := CAPIClusterInputs{Cluster: clusterTemplate}
	if raw, ok := data[CAPICredentialsTemplateKey]; ok {
		inputs.Credentials, err = newCredentialsTemplate(raw)
		if err != nil {
			return CAPIClusterInputs{}, fmt.Errorf("cloud provider %s: %w", registration.Type, err)
		}
	}
	return inputs, nil
}

func (s Source) LoadMCMInputs(ctx context.Context, provider Provider) (MCMInputs, error) {
	if err := provider.Registration.ValidateMCM(); err != nil {
		return MCMInputs{}, err
	}
	data, err := s.readTemplateSecret(ctx, provider.Registration.Type, engineMCM)
	if err != nil {
		return MCMInputs{}, err
	}
	inputs := MCMInputs{
		MachineClass: data[machineClassTemplateKey],
		Checksum:     data[machineClassChecksumKey],
		Config:       data[machineClassConfigKey],
	}
	if len(inputs.MachineClass) == 0 || len(inputs.Checksum) == 0 || len(inputs.Config) == 0 {
		return MCMInputs{}, fmt.Errorf("cloud provider %s publishes an incomplete MCM template set", provider.Registration.Type)
	}
	return inputs, nil
}

func (s Source) readClusterUUID(ctx context.Context) (string, error) {
	configMap := &corev1.ConfigMap{}
	if err := s.Reader.Get(ctx, types.NamespacedName{
		Name: clusterUUIDConfigMapName, Namespace: common.KubeSystemNamespace,
	}, configMap); err != nil {
		return "", fmt.Errorf("get cluster-uuid ConfigMap: %w", err)
	}
	clusterUUID := configMap.Data[clusterUUIDConfigMapKey]
	if clusterUUID == "" {
		return "", errors.New("cluster-uuid ConfigMap has no cluster-uuid")
	}
	return clusterUUID, nil
}

func (s Source) readTemplateSecret(ctx context.Context, cloudType, engine string) (map[string][]byte, error) {
	name := fmt.Sprintf("d8-cloud-provider-%s-%s", cloudType, engine)
	secret := &corev1.Secret{}
	if err := s.Reader.Get(ctx, types.NamespacedName{
		Name: name, Namespace: ProviderTemplateSecretNamespace,
	}, secret); err != nil {
		return nil, fmt.Errorf("get provider template Secret %s: %w", name, err)
	}
	return secret.Data, nil
}
