/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pki

import (
	"net"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfig_Defaults(t *testing.T) {
	cfg, err := newConfig("node", "cluster.local", net.ParseIP("10.0.0.1"), "10.96.0.0/12")
	require.NoError(t, err)

	assert.Equal(t, constants.DefaultCertificatesDir, cfg.pkiDir)
	assert.Equal(t, constants.EncryptionAlgorithmRSA2048, cfg.EncryptionAlgorithmType)
	assert.Equal(t, constants.CertificateValidityPeriod, cfg.CertValidityPeriod)
	assert.Equal(t, constants.CACertificateValidityPeriod, cfg.CACertValidityPeriod)
	assert.Equal(t, defaultCertTreeScheme, cfg.CertTreeScheme)
}

func TestNewConfig_RequiredFieldsMissing(t *testing.T) {
	validIP := net.ParseIP("10.0.0.1")

	tests := []struct {
		name            string
		nodeName        string
		dnsDomain       string
		advertiseAddr   net.IP
		serviceCIDR     string
		wantErrContains string
	}{
		{
			name:            "missing NodeName",
			nodeName:        "",
			dnsDomain:       "cluster.local",
			advertiseAddr:   validIP,
			serviceCIDR:     "10.96.0.0/12",
			wantErrContains: "NodeName",
		},
		{
			name:            "missing DNSDomain",
			nodeName:        "node",
			dnsDomain:       "",
			advertiseAddr:   validIP,
			serviceCIDR:     "10.96.0.0/12",
			wantErrContains: "DNSDomain",
		},
		{
			name:            "missing AdvertiseAddress",
			nodeName:        "node",
			dnsDomain:       "cluster.local",
			advertiseAddr:   nil,
			serviceCIDR:     "10.96.0.0/12",
			wantErrContains: "AdvertiseAddress",
		},
		{
			name:            "missing ServiceCIDR",
			nodeName:        "node",
			dnsDomain:       "cluster.local",
			advertiseAddr:   validIP,
			serviceCIDR:     "",
			wantErrContains: "ServiceCIDR",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newConfig(tc.nodeName, tc.dnsDomain, tc.advertiseAddr, tc.serviceCIDR)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErrContains)
		})
	}
}

func TestNewConfig_AllRequiredFieldsMissingReturnsAllErrors(t *testing.T) {
	_, err := newConfig("", "", nil, "")
	require.Error(t, err)

	msg := err.Error()
	assert.Contains(t, msg, "NodeName")
	assert.Contains(t, msg, "DNSDomain")
	assert.Contains(t, msg, "AdvertiseAddress")
	assert.Contains(t, msg, "ServiceCIDR")
}

func TestNewConfig_OptionsApplied(t *testing.T) {
	dir := t.TempDir()
	customSANs := []string{"my-lb.example.com", "10.10.10.10"}

	cfg, err := newConfig(
		"node",
		"cluster.local",
		net.ParseIP("10.0.0.1"),
		"10.96.0.0/12",
		WithPKIDir(dir),
		WithControlPlaneEndpoint("my-lb.example.com:6443"),
		WithAPIServerCertSANs(customSANs),
		WithEncryptionAlgorithmType(constants.EncryptionAlgorithmECDSAP256),
		WithCertValidityPeriod(30*24*time.Hour),
		WithCACertValidityPeriod(5*365*24*time.Hour),
	)
	require.NoError(t, err)

	assert.Equal(t, dir, cfg.pkiDir)
	assert.Equal(t, "my-lb.example.com:6443", cfg.ControlPlaneEndpoint)
	assert.Equal(t, customSANs, cfg.APIServerCertSANs)
	assert.Equal(t, constants.EncryptionAlgorithmECDSAP256, cfg.EncryptionAlgorithmType)
	assert.Equal(t, 30*24*time.Hour, cfg.CertValidityPeriod)
	assert.Equal(t, 5*365*24*time.Hour, cfg.CACertValidityPeriod)
}
