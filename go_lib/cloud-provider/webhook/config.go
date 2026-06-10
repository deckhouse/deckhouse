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

import "github.com/alecthomas/kingpin"

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

// InitFlags registers webhook server flags on the given kingpin application.
func InitFlags(cmd *kingpin.Application, cfg *ServerConfig) {
	if cfg == nil {
		return
	}
	cmd.Flag("webhook-port", "Webhook TLS server port").
		Envar("WEBHOOK_PORT").
		Default("8443").
		IntVar(&cfg.WebhookPort)
	cmd.Flag("webhook-cert-dir", "Directory with tls.crt and tls.key").
		Envar("WEBHOOK_CERT_DIR").
		Default("/certs").
		StringVar(&cfg.WebhookCertDir)
	cmd.Flag("metrics-bind-address", "Address for Prometheus metrics endpoint").
		Envar("METRICS_BIND_ADDRESS").
		Default(":8080").
		StringVar(&cfg.MetricsBindAddress)
	cmd.Flag("health-probe-bind-address", "Address for health probes").
		Envar("HEALTH_PROBE_BIND_ADDRESS").
		Default(":8081").
		StringVar(&cfg.HealthProbeBindAddress)
}
