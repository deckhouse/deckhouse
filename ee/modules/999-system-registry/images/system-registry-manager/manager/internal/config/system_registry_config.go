/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package config

type SystemRegistryConfig struct {
	NodeName string
	// MyIP string
}

func NewSystemRegistryConfig() (*SystemRegistryConfig, error) {
	return &SystemRegistryConfig{}, nil
}
