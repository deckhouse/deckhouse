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

package webhook

import (
	"testing"

	"github.com/alecthomas/kingpin"
)

func TestDefaultServerConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultServerConfig()
	if cfg.WebhookPort != 8443 || cfg.WebhookCertDir != "/certs" || cfg.MetricsBindAddress != ":8080" || cfg.HealthProbeBindAddress != ":8081" {
		t.Fatalf("DefaultServerConfig() = %#v, want defaults", cfg)
	}
}

func TestInitFlagsNilConfig(t *testing.T) {
	t.Parallel()

	InitFlags(kingpin.New("test", "test"), nil)
}

func TestInitFlagsPopulatesConfig(t *testing.T) {
	t.Parallel()

	cfg := ServerConfig{}
	app := kingpin.New("test", "test")
	InitFlags(app, &cfg)

	if _, err := app.Parse([]string{"--webhook-port=9443", "--webhook-cert-dir=/tmp/certs", "--metrics-bind-address=:9090", "--health-probe-bind-address=:9091"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if cfg.WebhookPort != 9443 || cfg.WebhookCertDir != "/tmp/certs" || cfg.MetricsBindAddress != ":9090" || cfg.HealthProbeBindAddress != ":9091" {
		t.Fatalf("InitFlags() cfg = %#v, want parsed values", cfg)
	}
}
