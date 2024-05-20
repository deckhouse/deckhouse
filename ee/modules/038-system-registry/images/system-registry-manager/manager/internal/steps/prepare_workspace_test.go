/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package steps

import (
	"github.com/stretchr/testify/assert"
	"os"
	"system-registry-manager/internal/config"
	pkg_files "system-registry-manager/pkg/files"
	"testing"
)

func TestRenderTemplate(t *testing.T) {

	err := config.InitConfigForTests(config.FileConfig{
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
		Distribution: struct {
			Image string `mapstructure:"image"`
		}{
			Image: "distribution_image",
		},
		Auth: struct {
			Image string `mapstructure:"image"`
		}{
			Image: "auth_image",
		},
		Seaweedfs: struct {
			Image string `mapstructure:"image"`
		}{
			Image: "seaweedfs_image",
		},
	})
	assert.NoError(t, err)

	manifestsSpec := config.NewManifestsSpecForTest()
	for _, manifest := range manifestsSpec.Manifests {
		manifestTemplate, err := os.ReadFile(manifest.InputPath)
		assert.NoError(t, err)

		_, err = pkg_files.RenderTemplate(string(manifestTemplate), config.GetDataForManifestRendering())
		assert.NoError(t, err)
	}
}
