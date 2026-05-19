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

package main

import (
	"os"
	"testing"
)

func TestParseArgs(t *testing.T) {
	t.Run("valid install mode", func(t *testing.T) {
		cli, err := parseArgs([]string{"install", "pkg:sha256:abc"})
		if err != nil {
			t.Fatalf("parseArgs() error = %v", err)
		}
		if cli.mode != modeInstall {
			t.Errorf("mode = %q, want %q", cli.mode, modeInstall)
		}
		if len(cli.packages) != 1 || cli.packages[0] != "pkg:sha256:abc" {
			t.Errorf("packages = %v, want [pkg:sha256:abc]", cli.packages)
		}
	})

	t.Run("valid fetch mode", func(t *testing.T) {
		cli, err := parseArgs([]string{"fetch"})
		if err != nil {
			t.Fatalf("parseArgs() error = %v", err)
		}
		if cli.mode != modeFetch {
			t.Errorf("mode = %q, want %q", cli.mode, modeFetch)
		}
	})

	t.Run("valid uninstall mode", func(t *testing.T) {
		cli, err := parseArgs([]string{"uninstall", "mypkg"})
		if err != nil {
			t.Fatalf("parseArgs() error = %v", err)
		}
		if cli.mode != modeUninstall {
			t.Errorf("mode = %q, want %q", cli.mode, modeUninstall)
		}
	})

	t.Run("unknown mode", func(t *testing.T) {
		_, err := parseArgs([]string{"deploy", "pkg:sha256:abc"})
		if err == nil {
			t.Fatal("parseArgs() error = nil, want error for unknown mode")
		}
	})

	t.Run("no args", func(t *testing.T) {
		_, err := parseArgs(nil)
		if err == nil {
			t.Fatal("parseArgs() error = nil, want error for empty args")
		}
	})

	t.Run("force flag default is false", func(t *testing.T) {
		cli, err := parseArgs([]string{"install"})
		if err != nil {
			t.Fatalf("parseArgs() error = %v", err)
		}
		if cli.force {
			t.Error("force = true, want false by default")
		}
	})

	t.Run("force flag enabled", func(t *testing.T) {
		cli, err := parseArgs([]string{"install", "--force"})
		if err != nil {
			t.Fatalf("parseArgs() error = %v", err)
		}
		if !cli.force {
			t.Error("force = false, want true when --force passed")
		}
	})
}

func TestParseEndpoints(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "single", input: "1.2.3.4:4219", want: []string{"1.2.3.4:4219"}},
		{name: "multiple", input: "1.2.3.4:4219,5.6.7.8:4219", want: []string{"1.2.3.4:4219", "5.6.7.8:4219"}},
		{name: "whitespace trimmed", input: " 1.2.3.4:4219 , 5.6.7.8:4219 ", want: []string{"1.2.3.4:4219", "5.6.7.8:4219"}},
		{name: "empty string", input: "", want: []string{}},
		{name: "only commas", input: ",,,", want: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseEndpoints(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("parseEndpoints(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseEndpoints(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestResolveEndpointsEnvEmpty(t *testing.T) {
	t.Setenv("PACKAGES_PROXY_ADDRESSES", "")

	endpoints, configured := resolveEndpoints("")
	if configured {
		t.Errorf("resolveEndpoints: configured = true for empty env, want false")
	}
	if len(endpoints) != 0 {
		t.Errorf("resolveEndpoints: endpoints = %v, want empty", endpoints)
	}
}

func TestResolveEndpointsEnvWhitespace(t *testing.T) {
	t.Setenv("PACKAGES_PROXY_ADDRESSES", "   ")

	endpoints, configured := resolveEndpoints("")
	if configured {
		t.Errorf("resolveEndpoints: configured = true for whitespace-only env, want false")
	}
	if len(endpoints) != 0 {
		t.Errorf("resolveEndpoints: endpoints = %v, want empty", endpoints)
	}
}

func TestResolveEndpointsEnvSet(t *testing.T) {
	t.Setenv("PACKAGES_PROXY_ADDRESSES", "1.2.3.4:4219,5.6.7.8:4219")

	endpoints, configured := resolveEndpoints("")
	if !configured {
		t.Fatal("resolveEndpoints: configured = false, want true")
	}
	if len(endpoints) != 2 {
		t.Errorf("resolveEndpoints: got %v, want 2 endpoints", endpoints)
	}
}

func TestResolveEndpointsFlagTakesPrecedenceOverEnv(t *testing.T) {
	t.Setenv("PACKAGES_PROXY_ADDRESSES", "1.2.3.4:4219")

	endpoints, configured := resolveEndpoints("9.9.9.9:4219")
	if !configured {
		t.Fatal("resolveEndpoints: configured = false, want true")
	}
	if len(endpoints) != 1 || endpoints[0] != "9.9.9.9:4219" {
		t.Errorf("resolveEndpoints: got %v, want [9.9.9.9:4219]", endpoints)
	}
}

func TestResolveTokenEnvEmpty(t *testing.T) {
	t.Setenv("PACKAGES_PROXY_TOKEN", "")

	token, configured := resolveToken("")
	if configured {
		t.Errorf("resolveToken: configured = true for empty env, want false")
	}
	if token != "" {
		t.Errorf("resolveToken: token = %q, want empty", token)
	}
}

func TestResolveTokenEnvWhitespace(t *testing.T) {
	t.Setenv("PACKAGES_PROXY_TOKEN", "   ")

	token, configured := resolveToken("")
	if configured {
		t.Errorf("resolveToken: configured = true for whitespace-only env, want false")
	}
	if token != "" {
		t.Errorf("resolveToken: token = %q, want empty", token)
	}
}

func TestResolveTokenEnvSet(t *testing.T) {
	t.Setenv("PACKAGES_PROXY_TOKEN", "mytoken")

	token, configured := resolveToken("")
	if !configured {
		t.Fatal("resolveToken: configured = false, want true")
	}
	if token != "mytoken" {
		t.Errorf("resolveToken: token = %q, want %q", token, "mytoken")
	}
}

func TestResolveTokenEnvTrimmed(t *testing.T) {
	t.Setenv("PACKAGES_PROXY_TOKEN", "  mytoken  ")

	token, configured := resolveToken("")
	if !configured {
		t.Fatal("resolveToken: configured = false, want true")
	}
	if token != "mytoken" {
		t.Errorf("resolveToken: token = %q, want %q", token, "mytoken")
	}
}

func TestResolveTokenUnsetEnvFallsThrough(t *testing.T) {
	os.Unsetenv("PACKAGES_PROXY_TOKEN") //nolint:errcheck

	_, configured := resolveToken("")
	if configured {
		t.Error("resolveToken: configured = true when env not set and no flag, want false")
	}
}

func TestResolveKubeAPIServerEndpointsEnvEmpty(t *testing.T) {
	t.Setenv("PACKAGES_PROXY_KUBE_APISERVER_ENDPOINTS", "")

	value, configured := resolveKubeAPIServerEndpoints("")
	if configured {
		t.Errorf("resolveKubeAPIServerEndpoints: configured = true for empty env, want false")
	}
	if value != "" {
		t.Errorf("resolveKubeAPIServerEndpoints: value = %q, want empty", value)
	}
}

func TestResolveKubeAPIServerEndpointsEnvSet(t *testing.T) {
	t.Setenv("PACKAGES_PROXY_KUBE_APISERVER_ENDPOINTS", "10.0.0.1:6443,10.0.0.2:6443")

	value, configured := resolveKubeAPIServerEndpoints("")
	if !configured {
		t.Fatal("resolveKubeAPIServerEndpoints: configured = false, want true")
	}
	if value != "10.0.0.1:6443,10.0.0.2:6443" {
		t.Errorf("resolveKubeAPIServerEndpoints: value = %q, want %q", value, "10.0.0.1:6443,10.0.0.2:6443")
	}
}

func TestResolveKubeAPIServerEndpointsFlagTakesPrecedenceOverEnv(t *testing.T) {
	t.Setenv("PACKAGES_PROXY_KUBE_APISERVER_ENDPOINTS", "10.0.0.1:6443")

	value, configured := resolveKubeAPIServerEndpoints("10.0.0.9:6443")
	if !configured {
		t.Fatal("resolveKubeAPIServerEndpoints: configured = false, want true")
	}
	if value != "10.0.0.9:6443" {
		t.Errorf("resolveKubeAPIServerEndpoints: value = %q, want %q", value, "10.0.0.9:6443")
	}
}
