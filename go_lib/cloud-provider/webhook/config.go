// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package webhook provides a controller-runtime admission webhook server skeleton
// for cloud-provider modules.
package webhook

import (
	"os"
	"strconv"

	"github.com/spf13/pflag"
)

// ServerConfig holds runtime settings for the validation webhook server.
type ServerConfig struct {
	// WebhookPort is the TLS port for admission webhook requests.
	WebhookPort int
	// WebhookCertDir is the directory with tls.crt and tls.key.
	WebhookCertDir string
	// MetricsBindAddress is the Prometheus metrics endpoint address.
	MetricsBindAddress string
	// HealthProbeBindAddress is the address for liveness and readiness probes.
	HealthProbeBindAddress string
}

// DefaultServerConfig returns the default webhook server configuration.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		WebhookPort:            8443,
		WebhookCertDir:         "/certs",
		MetricsBindAddress:     ":8080",
		HealthProbeBindAddress: ":8081",
	}
}

// InitServerFlags registers webhook server flags on the given pflag FlagSet.
func InitServerFlags(fs *pflag.FlagSet, cfg *ServerConfig) {
	if cfg == nil || fs == nil {
		return
	}

	if portRaw, ok := os.LookupEnv("WEBHOOK_PORT"); ok {
		if port, err := strconv.Atoi(portRaw); err == nil {
			cfg.WebhookPort = port
		}
	}
	if certDir, ok := os.LookupEnv("WEBHOOK_CERT_DIR"); ok {
		cfg.WebhookCertDir = certDir
	}
	if metricAddr, ok := os.LookupEnv("METRICS_BIND_ADDRESS"); ok {
		cfg.MetricsBindAddress = metricAddr
	}
	if healthProbeAddr, ok := os.LookupEnv("HEALTH_PROBE_BIND_ADDRESS"); ok {
		cfg.HealthProbeBindAddress = healthProbeAddr
	}

	fs.IntVar(&cfg.WebhookPort, "webhook-port", 8443, "Webhook TLS server port")
	fs.StringVar(&cfg.WebhookCertDir, "webhook-cert-dir", "/certs", "Directory with tls.crt and tls.key")
	fs.StringVar(&cfg.MetricsBindAddress, "metrics-bind-address", ":8080", "Address for Prometheus metrics endpoint")
	fs.StringVar(&cfg.HealthProbeBindAddress, "health-probe-bind-address", ":8081", "Address for health probes")
}
