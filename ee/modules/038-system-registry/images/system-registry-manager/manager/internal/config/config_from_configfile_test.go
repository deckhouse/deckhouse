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
	_, err = io.WriteString(tmpFile, ``)
	assert.NoError(t, err)
	tmpFile.Close()

	// Set up test environment variables
	os.Setenv("HOSTNAME", "testhostname")
	os.Setenv("MY_IP", "testmyip")
	os.Setenv("MY_POD_NAME", "testpodname")
	os.Setenv("LEADER_ELECTION_NAMESPACE", "testnamespace")
	os.Setenv("LEADER_ELECTION_LEASE_DURATION_SECONDS", "3600")
	os.Setenv("LEADER_ELECTION_RENEW_DEADLINE_SECONDS", "10")
	os.Setenv("LEADER_ELECTION_RETRY_PERIOD_SECONDS", "4")

	// Run the function under test
	cfg, err := NewFileConfig()

	// Assert that no error occurred
	assert.NoError(t, err)

	// Assert the values in the config
	assert.Equal(t, "testhostname", cfg.HostName)
	assert.Equal(t, "testmyip", cfg.MyIP)
	assert.Equal(t, "testpodname", cfg.MyPodName)
	assert.Equal(t, "testnamespace", cfg.LeaderElection.Namespace)
	assert.Equal(t, 3600, cfg.LeaderElection.LeaseDurationSeconds)
	assert.Equal(t, 10, cfg.LeaderElection.RenewDeadlineSeconds)
	assert.Equal(t, 4, cfg.LeaderElection.RetryPeriodSeconds)
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
myIP: filemyip
myPodName: filepodname
leaderElection:
  namespace: filenamespace
  leaseDurationSeconds: 7200
  renewDeadlineSeconds: 20
  retryPeriodSeconds: 8
`)
	assert.NoError(t, err)
	tmpFile.Close()

	// Run the function under test
	cfg, err := NewFileConfig()

	// Assert that no error occurred
	assert.NoError(t, err)

	// Assert the values in the config
	assert.Equal(t, "filehostname", cfg.HostName)
	assert.Equal(t, "filemyip", cfg.MyIP)
	assert.Equal(t, "filepodname", cfg.MyPodName)
	assert.Equal(t, "filenamespace", cfg.LeaderElection.Namespace)
	assert.Equal(t, 7200, cfg.LeaderElection.LeaseDurationSeconds)
	assert.Equal(t, 20, cfg.LeaderElection.RenewDeadlineSeconds)
	assert.Equal(t, 8, cfg.LeaderElection.RetryPeriodSeconds)
}
