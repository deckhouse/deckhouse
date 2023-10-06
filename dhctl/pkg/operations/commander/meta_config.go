package commander

import (
	"fmt"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

func ParseMetaConfig(stateCache state.Cache, clusterConfigurationData, providerClusterConfigurationData []byte) (*config.MetaConfig, error) {
	clusterUUIDBytes, err := stateCache.Load("uuid")
	if err != nil {
		return nil, fmt.Errorf("error loading cluster uuid from state cache: %w", err)
	}
	clusterUUID := string(clusterUUIDBytes)

	configData := fmt.Sprintf("%s\n---\n%s", clusterConfigurationData, providerClusterConfigurationData)
	metaConfig, err := config.ParseConfigFromData(configData)
	if err != nil {
		return nil, fmt.Errorf("unable to parse config: %w", err)
	}
	metaConfig.UUID = clusterUUID

	return metaConfig, nil
}
