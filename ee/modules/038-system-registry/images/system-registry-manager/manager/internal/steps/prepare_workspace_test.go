/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package steps

import (
	"github.com/stretchr/testify/assert"
	"system-registry-manager/internal/config"
	"testing"
)

func generateInputConfig() error {
	return config.InitConfigForTests(config.FileConfig{
		HostName: "filehostname",
		HostIP:   "filemyip",
		PodName:  "filepodname",
		LeaderElection: struct {
			Namespace            string "mapstructure:\"namespace\""
			LeaseDurationSeconds int    "mapstructure:\"leaseDurationSeconds\""
			RenewDeadlineSeconds int    "mapstructure:\"renewDeadlineSeconds\""
			RetryPeriodSeconds   int    "mapstructure:\"retryPeriodSeconds\""
		}{
			Namespace:            "filenamespace",
			LeaseDurationSeconds: 7200,
			RenewDeadlineSeconds: 20,
			RetryPeriodSeconds:   8,
		},
		// Add new fields
		Etcd: struct {
			Addresses []string `mapstructure:"addresses"`
		}{
			Addresses: []string{"etcd1.example.com", "etcd2.example.com"},
		},
		Images: struct {
			SystemRegistry struct {
				DockerDistribution string "mapstructure:\"dockerDistribution\""
				DockerAuth         string "mapstructure:\"dockerAuth\""
				Seaweedfs          string "mapstructure:\"seaweedfs\""
			} "mapstructure:\"systemRegistry\""
		}{
			SystemRegistry: struct {
				DockerDistribution string "mapstructure:\"dockerDistribution\""
				DockerAuth         string "mapstructure:\"dockerAuth\""
				Seaweedfs          string "mapstructure:\"seaweedfs\""
			}{
				DockerDistribution: "distribution_image",
				DockerAuth:         "auth_image",
				Seaweedfs:          "seaweedfs_image",
			},
		},
	})
}

func TestCheckInputManifestsExist(t *testing.T) {
	err := generateInputConfig()
	assert.NoError(t, err)

	manifestsSpec := config.NewManifestsSpecForTest()
	err = checkInputManifestsExist(manifestsSpec)
	assert.NoError(t, err)
}

func TestCopyManifestsToWorkspace(t *testing.T) {
	err := generateInputConfig()
	assert.NoError(t, err)

	manifestsSpec := config.NewManifestsSpecForTest()
	err = copyManifestsToWorkspace(manifestsSpec)
	assert.NoError(t, err)
}
