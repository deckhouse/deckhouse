package config

type SystemRegistryConfig struct {
	NodeName string
	// MyIP string
}

func NewSystemRegistryConfig() (*SystemRegistryConfig, error) {
	return &SystemRegistryConfig{}, nil
}
