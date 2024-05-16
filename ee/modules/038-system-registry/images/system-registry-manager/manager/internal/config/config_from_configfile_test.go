/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package config

import (
	"github.com/stretchr/testify/assert"
	"io"
	"os"
	"testing"
)

func TestNewFileConfig_WithEnv(t *testing.T) {
	// Create a temporary config file
	tmpFile, err := os.CreateTemp("", "testconfig.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	SetConfigFilePath(tmpFile.Name())

	// Write test data to the temp config file
	_, err = io.WriteString(tmpFile, `
leaderElection:
  namespace: filenamespace
  leaseDurationSeconds: 7200
  renewDeadlineSeconds: 20
  retryPeriodSeconds: 8
etcd:
  addresses: ["etcd1.example.com", "etcd2.example.com"]
distribution:
  image: distribution_image
auth:
  image: auth_image
seaweedfs:
  image: seaweedfs_image
`)
	assert.NoError(t, err)
	tmpFile.Close()

	// Set up test environment variables
	os.Setenv("HOSTNAME", "filehostname")
	os.Setenv("HOST_IP", "filemyip") // Correct the environment variable name
	os.Setenv("POD_NAME", "filepodname")

	// Restore the original environment variables after the test
	defer func() {
		os.Unsetenv("HOSTNAME")
		os.Unsetenv("HOST_IP")
		os.Unsetenv("POD_NAME")
	}()

	// Run the function under test
	expectedCfg := &FileConfig{
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
	}
	cfg, err := NewFileConfig()

	// Assert that no error occurred
	assert.NoError(t, err)

	// Assert the entire config structure
	assert.Equal(t, expectedCfg, cfg)
}

func TestNewFileConfig_WithFile(t *testing.T) {
	// Create a temporary config file
	tmpFile, err := os.CreateTemp("", "testconfig.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	SetConfigFilePath(tmpFile.Name())

	// Write test data to the temp config file
	_, err = io.WriteString(tmpFile, `
hostName: filehostname
hostIP: filemyip
podName: filepodname
leaderElection:
  namespace: filenamespace
  leaseDurationSeconds: 7200
  renewDeadlineSeconds: 20
  retryPeriodSeconds: 8
etcd:
  addresses: ["etcd1.example.com", "etcd2.example.com"]
distribution:
  image: distribution_image
auth:
  image: auth_image
seaweedfs:
  image: seaweedfs_image
`)
	assert.NoError(t, err)
	tmpFile.Close()

	// Run the function under test
	expectedCfg := &FileConfig{
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
	}
	cfg, err := NewFileConfig()

	// Assert that no error occurred
	assert.NoError(t, err)

	// Assert the entire config structure
	assert.Equal(t, expectedCfg, cfg)
}
