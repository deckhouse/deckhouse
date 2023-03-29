package main

import (
	"os"
	"testing"
)

func TestCalculateConfigurationChecksum(t *testing.T) {
	os.Setenv("TESTS_CONFIG_PATH", "testdata/config")
	err := calculateConfigurationChecksum()
	if err != nil {
		t.Fatal(err)
	}

	if configurationChecksum == "" {
		t.Fatal("calculated configuration checksum is not calculated")
	}
}
