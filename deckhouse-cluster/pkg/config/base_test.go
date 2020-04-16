package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestParseConfig(t *testing.T) {
	mockPath, _ := filepath.Abs("./mock/openstack")
	os.Setenv("MODULES_DIR", mockPath)

	tests := []struct {
		name    string
		test    func() error
		wantErr bool
	}{
		{
			"Valid config",
			func() error {
				_, err := ParseConfig(mockPath + "/config.yaml")
				return err
			},
			false,
		},
		{
			"Invalid config",
			func() error {
				_, err := ParseConfig(mockPath + "/invalid-config.yaml")
				return err
			},
			true,
		},
		{
			"Defaults test",
			func() error {
				metaConfig, err := ParseConfig(mockPath + "/config.yaml")
				if err != nil {
					return err
				}
				metaConfig.PrepareBootstrapSettings()

				bundle := metaConfig.BootstrapConfig.Deckhouse.Bundle
				if bundle != "Default" {
					return fmt.Errorf("expect bundle to be Default, got %s", bundle)
				}
				return nil
			},
			false,
		},
	}

	for _, tc := range tests {
		err := tc.test()

		if err != nil && !tc.wantErr {
			t.Errorf("%s: %v", tc.name, err)
		}

		if err == nil && tc.wantErr {
			t.Errorf("%s: expected error, didn't get one", tc.name)
		}
	}
}
