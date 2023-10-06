package commander

type CommanderModeParams struct {
	ClusterConfigurationData         []byte
	ProviderClusterConfigurationData []byte
}

func NewCommanderModeParams(clusterConfigurationData, providerClusterConfigurationData []byte) *CommanderModeParams {
	if clusterConfigurationData == nil {
		panic("cluster configuration param required")
	}
	if providerClusterConfigurationData == nil {
		panic("provider cluster configuration param required")
	}
	return &CommanderModeParams{
		ClusterConfigurationData:         clusterConfigurationData,
		ProviderClusterConfigurationData: providerClusterConfigurationData,
	}
}
