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

package kubeclient

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewRestConfigLimitsTheDial(t *testing.T) {
	kubeconfig := filepath.Join(t.TempDir(), "config")
	content := `apiVersion: v1
kind: Config
clusters:
- name: test
  cluster:
    server: https://127.0.0.1:6443
contexts:
- name: test
  context:
    cluster: test
current-context: test
`
	if err := os.WriteFile(kubeconfig, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", kubeconfig)

	cfg, err := NewRestConfig()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Dial == nil {
		t.Fatal("the config must have a dial with a short timeout")
	}

	if dialer.Timeout != 5*time.Second {
		t.Fatalf("dial timeout = %s, want 5s", dialer.Timeout)
	}

	// The dial connects as the client-go default does.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	conn, err := cfg.Dial(t.Context(), "tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
}
