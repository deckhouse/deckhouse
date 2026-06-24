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

package containerd

import "testing"

func TestRenderHostsTOML_PrimaryWithFailover(t *testing.T) {
	host := HostConfig{
		Server: "registry.d8-system.svc:5001",
		Entries: []HostEntry{
			{URL: "https://127.0.0.1:5001", Capabilities: []string{"pull", "resolve"}, CA: "AGENTCA"},
			{URL: "https://10.0.0.1:5001", Capabilities: []string{"pull", "resolve"}, CA: "CACHECA", Auth: "dXNlcjpwYXNz"},
		},
	}
	got, err := renderHostsTOML(host, []string{"/etc/containerd/registry.d/h/0.crt", "/etc/containerd/registry.d/h/1.crt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `server = "registry.d8-system.svc:5001"

[host."https://127.0.0.1:5001"]
  capabilities = ["pull", "resolve"]
  ca = ["/etc/containerd/registry.d/h/0.crt"]

[host."https://10.0.0.1:5001"]
  capabilities = ["pull", "resolve"]
  ca = ["/etc/containerd/registry.d/h/1.crt"]
  [host."https://10.0.0.1:5001".auth]
    auth = "dXNlcjpwYXNz"
`
	if got != want {
		t.Fatalf("rendered TOML mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderHostsTOML_HTTPSkipVerifyNoCA(t *testing.T) {
	host := HostConfig{
		Server: "docker.io",
		Entries: []HostEntry{
			{URL: "http://127.0.0.1:5001", Capabilities: []string{"pull"}, SkipVerify: true},
		},
	}
	got, err := renderHostsTOML(host, []string{""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `server = "docker.io"

[host."http://127.0.0.1:5001"]
  capabilities = ["pull"]
  skip_verify = true
`
	if got != want {
		t.Fatalf("rendered TOML mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderHostsTOML_CAPathsLengthMismatch(t *testing.T) {
	host := HostConfig{Server: "x", Entries: []HostEntry{{URL: "https://a"}}}
	if _, err := renderHostsTOML(host, nil); err == nil {
		t.Fatal("expected error on caPaths length mismatch, got nil")
	}
}
