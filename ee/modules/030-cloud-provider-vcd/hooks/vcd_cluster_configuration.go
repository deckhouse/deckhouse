/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/json"
	"errors"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/vcd"
	v1 "github.com/deckhouse/deckhouse/ee/modules/030-cloud-provider-vcd/hooks/internal/v1"
	cloudDataV1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
	"github.com/deckhouse/deckhouse/go_lib/hooks/cluster_configuration"
)

func preparatorProvider(_ string) config.MetaConfigPreparator {
	return vcd.NewMetaConfigPreparatorWithoutLogger(
		vcd.MetaConfigPreparatorParams{
			// todo it was bad idea patch metaconfig during installation
			// we need to prepare meta config in dhctl during installation
			// for checking vcd version
			// we do not want to prepare metaconfig here because we already got prepared
			// after installation during first deckhouse converge and legacy mode will get
			// in legacy_mode.go hook with order 10, this hook has 20 order index
			// after first converge we deploy vcd data discoverer and hook legacy_mode.go
			// directory form data discoverer
			PrepareMetaConfig: false,
			// cluster prefix does not provide here
			ValidateClusterPrefix: false,
		},
	)
}

var _ = cluster_configuration.RegisterHook(
	func(
		input *go_hook.HookInput, metaCfg *config.MetaConfig, providerDiscoveryData *unstructured.Unstructured,
		_ bool,
	) error {
		p := make(map[string]json.RawMessage)
		if metaCfg != nil {
			p = metaCfg.ProviderClusterConfig
		}

		var providerClusterConfiguration v1.VCDProviderClusterConfiguration
		err := convertJSONRawMessageToStruct(p, &providerClusterConfiguration)
		if err != nil {
			return err
		}

		var moduleConfig v1.VCDModuleConfig
		err = json.Unmarshal([]byte(input.Values.Get("cloudProviderVcd").String()), &moduleConfig)
		if err != nil {
			return err
		}

		overrideProviderClusterConfig(&providerClusterConfiguration, &moduleConfig)

		err = validateProviderClusterConfig(providerClusterConfiguration)
		if err != nil {
			return err
		}
		input.Values.Set("cloudProviderVcd.internal.providerClusterConfiguration", providerClusterConfiguration)

		var discoveryData cloudDataV1.VCDCloudProviderDiscoveryData
		if providerDiscoveryData != nil {
			err := sdk.FromUnstructured(providerDiscoveryData, &discoveryData)
			if err != nil {
				return err
			}
		}

		providerDiscoveryDataValuesJSON, ok := input.Values.GetOk("cloudProviderVcd.internal.providerDiscoveryData")
		if ok && len(providerDiscoveryDataValuesJSON.String()) != 0 {
			var currentDiscoveryData cloudDataV1.VCDCloudProviderDiscoveryData
			err = json.Unmarshal([]byte(providerDiscoveryDataValuesJSON.String()), &currentDiscoveryData)
			if err != nil {
				return err
			}

			discoveryData = mergeDiscoveryData(currentDiscoveryData, discoveryData)
		}

		input.Values.Set("cloudProviderVcd.internal.providerDiscoveryData", discoveryData)

		return nil
	}, cluster_configuration.NewConfig(preparatorProvider),
)

func convertJSONRawMessageToStruct(in map[string]json.RawMessage, out interface{}) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func overrideProviderClusterConfig(
	providerClusterConfig *v1.VCDProviderClusterConfiguration,
	moduleConfig *v1.VCDModuleConfig,
) {
	if moduleConfig.Provider != nil && !moduleConfig.Provider.IsEmpty() {
		if providerClusterConfig.Provider == nil {
			providerClusterConfig.Provider = &v1.VCDProvider{}
		}
		if moduleConfig.Provider.Server != nil {
			providerClusterConfig.Provider.Server = moduleConfig.Provider.Server
		}
		if moduleConfig.Provider.Username != nil {
			providerClusterConfig.Provider.Username = moduleConfig.Provider.Username
		}
		if moduleConfig.Provider.Password != nil {
			providerClusterConfig.Provider.Password = moduleConfig.Provider.Password
		}
		if moduleConfig.Provider.APIToken != nil {
			providerClusterConfig.Provider.APIToken = moduleConfig.Provider.APIToken
		}
		if moduleConfig.Provider.Insecure != nil {
			providerClusterConfig.Provider.Insecure = moduleConfig.Provider.Insecure
		}
	}
	if len(moduleConfig.Organization) > 0 {
		providerClusterConfig.Organization = &moduleConfig.Organization
	}
	if len(moduleConfig.VirtualDataCenter) > 0 {
		providerClusterConfig.VirtualDataCenter = &moduleConfig.VirtualDataCenter
	}
	if len(moduleConfig.VirtualApplicationName) > 0 {
		providerClusterConfig.VirtualApplicationName = &moduleConfig.VirtualApplicationName
	}
	if len(moduleConfig.MainNetwork) > 0 {
		providerClusterConfig.MainNetwork = &moduleConfig.MainNetwork
	}
	if len(moduleConfig.SSHPublicKey) > 0 {
		providerClusterConfig.SSHPublicKey = &moduleConfig.SSHPublicKey
	}
	if len(moduleConfig.Metadata) > 0 {
		providerClusterConfig.Metadata = moduleConfig.Metadata
	}

	if len(providerClusterConfig.Metadata) == 0 {
		providerClusterConfig.Metadata = make(map[string]string)
	}
}

func validateProviderClusterConfig(providerClusterConfig v1.VCDProviderClusterConfiguration) error {
	if providerClusterConfig.Provider == nil {
		return errors.New("provider section is required")
	}

	hasAPIToken := providerClusterConfig.Provider.APIToken != nil && len(*providerClusterConfig.Provider.APIToken) > 0
	hasUsername := providerClusterConfig.Provider.Username != nil && len(*providerClusterConfig.Provider.Username) > 0
	hasPassword := providerClusterConfig.Provider.Password != nil && len(*providerClusterConfig.Provider.Password) > 0
	hasUserPass := hasUsername || hasPassword

	if hasAPIToken && hasUserPass {
		return errors.New("provider authentication must use either apiToken or username/password")
	}
	if !hasAPIToken && !hasUserPass {
		return errors.New("provider.apiToken or provider.username/provider.password should be set")
	}
	if !hasAPIToken && (!hasUsername || !hasPassword) {
		return errors.New("both provider.username and provider.password should be set")
	}

	if providerClusterConfig.Provider.Server == nil || len(*providerClusterConfig.Provider.Server) == 0 {
		return errors.New("provider.server cannot be empty")
	}
	if providerClusterConfig.Organization == nil || len(*providerClusterConfig.Organization) == 0 {
		return errors.New("organization cannot be empty")
	}
	if providerClusterConfig.VirtualDataCenter == nil || len(*providerClusterConfig.VirtualDataCenter) == 0 {
		return errors.New("virtualDataCenter cannot be empty")
	}
	if providerClusterConfig.VirtualApplicationName == nil || len(*providerClusterConfig.VirtualApplicationName) == 0 {
		return errors.New("virtualApplicationName cannot be empty")
	}
	if providerClusterConfig.MainNetwork == nil || len(*providerClusterConfig.MainNetwork) == 0 {
		return errors.New("mainNetwork cannot be empty")
	}

	cloudManaged := providerClusterConfig.APIVersion != nil || providerClusterConfig.Kind != nil
	if cloudManaged {
		if providerClusterConfig.APIVersion == nil || len(*providerClusterConfig.APIVersion) == 0 {
			return errors.New("apiVersion cannot be empty")
		}
		if providerClusterConfig.Kind == nil || len(*providerClusterConfig.Kind) == 0 {
			return errors.New("kind cannot be empty")
		}
	}

	return nil
}

func mergeDiscoveryData(
	currentValue cloudDataV1.VCDCloudProviderDiscoveryData,
	newValue cloudDataV1.VCDCloudProviderDiscoveryData,
) cloudDataV1.VCDCloudProviderDiscoveryData {
	result := currentValue

	if newValue.APIVersion != "" && currentValue.APIVersion == "" {
		result.APIVersion = newValue.APIVersion
	}
	if newValue.Kind != "" && currentValue.Kind == "" {
		result.Kind = newValue.Kind
	}
	if len(newValue.SizingPolicies) > 0 && len(currentValue.SizingPolicies) == 0 {
		result.SizingPolicies = newValue.SizingPolicies
	}
	if len(newValue.StorageProfiles) > 0 && len(currentValue.StorageProfiles) == 0 {
		result.StorageProfiles = newValue.StorageProfiles
	}
	if len(newValue.InternalNetworks) > 0 && len(currentValue.InternalNetworks) == 0 {
		result.InternalNetworks = newValue.InternalNetworks
	}
	if len(newValue.Zones) > 0 && len(currentValue.Zones) == 0 {
		result.Zones = newValue.Zones
	}
	if newValue.VCDAPIVersion != "" && currentValue.VCDAPIVersion == "" {
		result.VCDAPIVersion = newValue.VCDAPIVersion
	}
	if newValue.VCDInstallationVersion != "" && currentValue.VCDInstallationVersion == "" {
		result.VCDInstallationVersion = newValue.VCDInstallationVersion
	}
	if newValue.LoadBalancer != nil && currentValue.LoadBalancer == nil {
		result.LoadBalancer = new(cloudDataV1.VCDLoadBalancer)
		result.LoadBalancer.Enabled = newValue.LoadBalancer.Enabled
	}

	if result.APIVersion == "" {
		result.APIVersion = "deckhouse.io/v1"
	}
	if result.Kind == "" {
		result.Kind = "VCDCloudProviderDiscoveryData"
	}
	if len(result.Zones) == 0 {
		result.Zones = []string{"default"}
	}

	return result
}
