package config

import (
	"fmt"
	"os"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

// SetEnvVars sets environment variables based on the configuration
func SetEnvVars(config *MetaConfig) error {
	if config.ProviderName == "vcd" {
		for _, moduleConfig := range config.ModuleConfigs {
			if moduleConfig.Name == "cloud-provider-vcd" {
				minVcdApiVersion, ok := moduleConfig.Spec.Settings["minVcdApiVersion"]
				if ok {
					minVcdApiVersion, ok := minVcdApiVersion.(string)
					if ok {
						log.DebugF("Setting GOVCD_API_VERSION to '%s'\n", minVcdApiVersion)

						err := os.Setenv("GOVCD_API_VERSION", minVcdApiVersion)
						if err != nil {
							return fmt.Errorf("failed to set GOVCD_API_VERSION env var: %v", err)
						}
					}
				}
			}
		}
	}

	return nil
}
