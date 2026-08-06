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

package manifests

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

// bashibleBootstrapAuthenticationConfig is what candi/bashible writes into
// /etc/kubernetes/deckhouse/extra-files/authentication-config.yaml before it copies the
// kube-apiserver manifest into place, quoted here byte for byte from the heredoc in
// 072_install_control_plane.sh.tpl.
//
// It is the minimum kube-apiserver needs to answer its own probes. If this package rendered
// anything else for a bootstrap, the classic path and the immutable path would bring the first
// control plane up with different authentication configuration.
const bashibleBootstrapAuthenticationConfig = `apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthenticationConfiguration
anonymous:
  enabled: true
  conditions:
  - path: /livez
  - path: /readyz
  - path: /healthz
`

func TestRenderExtraFilesMatchesBashibleOnBootstrap(t *testing.T) {
	bundle, err := RenderExtraFiles(t.Context(), bootstrapContext(nil), NodeInput{
		NodeName: "master-0",
		NodeIP:   "10.0.0.1",
	})
	if err != nil {
		t.Fatalf("render extra files: %v", err)
	}

	if len(bundle) != 1 {
		t.Fatalf("expected one extra file, got %d", len(bundle))
	}
	if bundle[0].Name != "authentication-config.yaml" {
		t.Fatalf("expected authentication-config.yaml, got %q", bundle[0].Name)
	}

	got := string(bundle[0].Content)
	if got != bashibleBootstrapAuthenticationConfig {
		t.Fatalf("bootstrap authentication config differs from bashible\n--- want ---\n%s\n--- got ---\n%s", bashibleBootstrapAuthenticationConfig, got)
	}
}

// TestRenderExtraFilesMatchesHelmDefine is the guard against the two copies drifting.
//
// The module's helm chart still owns this file on the day-2 path, so the same input has to
// produce the same bytes on both sides: the node writes it at bootstrap, control-plane-manager
// rewrites it from the Secret afterwards, and a difference means a rewrite of the file — and a
// restart of what reads it — on every cluster, minutes after it came up.
func TestRenderExtraFilesMatchesHelmDefine(t *testing.T) {
	definePath := findUp(t, filepath.Join("modules", "040-control-plane-manager", "templates", "_authentication_configuration.tpl"))
	if definePath == "" {
		t.Skip("helm define not found — the package is being tested outside the deckhouse tree")
	}

	raw, err := os.ReadFile(definePath)
	if err != nil {
		t.Fatalf("read helm define: %v", err)
	}

	helm := template.Must(template.New("helm").Funcs(funcMap()).Parse(string(raw)))

	cases := []struct {
		name string
		data map[string]any
	}{
		{"bootstrap, nothing configured", bootstrapContext(nil)},
		{"oidc issuer", bootstrapContext(map[string]any{
			"oidcIssuerURL": "https://dex.example.com/",
		})},
		{"oidc issuer with a CA", bootstrapContext(map[string]any{
			"oidcIssuerURL": "https://dex.example.com/",
			"oidcCA":        "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----",
		})},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var want bytes.Buffer
			if err := helm.ExecuteTemplate(&want, "authenticationConfiguration", tc.data); err != nil {
				t.Fatalf("render the helm define: %v", err)
			}

			bundle, err := RenderExtraFiles(t.Context(), tc.data, NodeInput{NodeName: "master-0", NodeIP: "10.0.0.1"})
			if err != nil {
				t.Fatalf("render extra files: %v", err)
			}

			// The define is wrapped in {{- define }}, which leaves a leading newline the file
			// itself does not have; compare what the two actually say.
			if strings.TrimSpace(want.String()) != strings.TrimSpace(string(bundle[0].Content)) {
				t.Fatalf("the package and the helm define disagree\n--- helm ---\n%s\n--- package ---\n%s", want.String(), bundle[0].Content)
			}
		})
	}
}

// TestRenderExtraFilesRefusesWhatItCannotWrite pins the fail-closed half: an input that makes a
// manifest reference a file this package does not render has to be an error here rather than a
// missing file on a node.
func TestRenderExtraFilesRefusesWhatItCannotWrite(t *testing.T) {
	cases := []struct {
		name string
		data map[string]any
		want string
	}{
		{"audit policy", bootstrapContext(map[string]any{"auditPolicy": "YXBpVmVyc2lvbjo="}), "audit-policy.yaml"},
		{"authorization webhook", bootstrapContext(map[string]any{"webhookURL": "https://authz.example.com"}), "authorization-config.yaml"},
		{"authn webhook", bootstrapContext(map[string]any{"authnWebhookURL": "https://authn.example.com"}), "authn-webhook-config.yaml"},
		{"audit webhook", bootstrapContext(map[string]any{"auditWebhookURL": "https://audit.example.com"}), "audit-webhook-config.yaml"},
		{"secret encryption", bootstrapContext(map[string]any{"secretEncryptionKey": "a2V5"}), "secret-encryption-config.yaml"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RenderExtraFiles(t.Context(), tc.data, NodeInput{NodeName: "master-0", NodeIP: "10.0.0.1"})
			if err == nil {
				t.Fatalf("expected a refusal naming %s", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("the refusal does not name %s: %v", tc.want, err)
			}
		})
	}

	t.Run("a normal run needs files only helm renders", func(t *testing.T) {
		data := bootstrapContext(nil)
		data["runType"] = "Normal"

		_, err := RenderExtraFiles(t.Context(), data, NodeInput{NodeName: "master-0", NodeIP: "10.0.0.1"})
		if err == nil {
			t.Fatal("expected a refusal: a normal run also needs admission-control-config and scheduler-config")
		}
		for _, want := range []string{"admission-control-config.yaml", "scheduler-config.yaml"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("the refusal does not name %s: %v", want, err)
			}
		}
	})
}

func bootstrapContext(apiserver map[string]any) map[string]any {
	if apiserver == nil {
		apiserver = map[string]any{}
	}
	return map[string]any{
		"runType":   RunTypeClusterBootstrap,
		"apiserver": apiserver,
		"clusterConfiguration": map[string]any{
			"kubernetesVersion": "1.34",
			"clusterDomain":     "cluster.local",
		},
	}
}

// findUp walks up from the working directory looking for rel, so the test finds the module's
// helm templates whether it runs from the package or from the repository root.
func findUp(t *testing.T, rel string) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for {
		candidate := filepath.Join(dir, rel)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
