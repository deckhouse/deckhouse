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

package kubernetes

import (
	"testing"

	"kubernetes-api-proxy/internal/config"
)

func TestBuildProbeConfigGetter_StaticPod(t *testing.T) {
	getter := BuildProbeConfigGetter(config.Config{AsStaticPod: true})

	cfg, err := getter()
	if err != nil {
		t.Fatalf("getter() error = %v", err)
	}

	if cfg.Host != "" {
		t.Errorf("Host = %q, want empty", cfg.Host)
	}

	got := cfg.TLSClientConfig
	if got.CAFile != caFile || got.CertFile != certFile || got.KeyFile != keyFile {
		t.Errorf("TLSClientConfig = %+v, want files %s %s %s", got, caFile, certFile, keyFile)
	}
}
