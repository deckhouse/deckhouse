/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"sigs.k8s.io/yaml"
)

// TestSanitizeZoneParity guards against drift between the Go implementation of
// rfc1123SubdomainName in this package and the sprig sanitization block embedded in
// capi/template.yaml under machineDeployment.additionalFields.failureDomain.
//
// The template's failureDomain has two branches:
//
//   - baseline: `{{ if not (hasKey .instanceClass "resourcePool") }}<sanZone>{{ end }}`
//   - override: `{{ if hasKey .instanceClass "resourcePool" }}<sanZone>-<sanNG>{{ end }}`
//
// Both must produce the same string CAPV's generateOverrideFunc uses to look up
// VSphereDeploymentZone. This hook writes the DZ name through rfc1123SubdomainName; the
// template writes Machine.Spec.FailureDomain through the sprig block. A silent divergence
// causes CAPV's per-zone resourcePool / folder override to no-op — VMs clone with only
// the baseline topology from the template and end up in the wrong resource pool,
// invisibly from kubectl.
//
// The sprig expression is read from template.yaml as text (not hardcoded), so a change
// to that file is exercised on the next run without needing to also touch this test.
// The Funcs set is sprig.TxtFuncMap: node-controller renders the field through the
// same map (its sandbox removes only clock/random/crypto/env — every function used here
// is pure and stays in). If a future sandbox change removes lower / replace /
// regexReplaceAll / trimAll, the golden SandboxFuncNames test in
// internal/machinetemplate/sandbox_test.go catches that separately.
func TestSanitizeZoneParity(t *testing.T) {
	sprigExpr := readSprigFailureDomainExpr(t)

	tmpl, err := template.New("failureDomain").
		Funcs(sprig.TxtFuncMap()).
		Option("missingkey=error").
		Parse(sprigExpr)
	if err != nil {
		t.Fatalf("parse sprig expression from template.yaml: %v", err)
	}

	cases := []string{
		"e2e-zone-1",
		"E2E Zone 1",
		"Prod-Zone.a",
		"zone_a",
		"  leading-trailing  ",
		"..dots..",
		"---",
		"a/b:c",
		"",
	}

	// Baseline branch: no resourcePool on InstanceClass → template renders sanitized zone
	// alone, hook uses the same value as DZ name.
	t.Run("baseline_no_resourcePool", func(t *testing.T) {
		for _, in := range cases {
			t.Run(in, func(t *testing.T) {
				var buf bytes.Buffer
				ctx := map[string]any{
					"zone":          in,
					"instanceClass": map[string]any{}, // no resourcePool key
					"nodeGroup":     map[string]any{"name": "worker-fast"},
				}
				if err := tmpl.Execute(&buf, ctx); err != nil {
					t.Fatalf("execute baseline sprig on zone=%q: %v", in, err)
				}
				got := buf.String()
				want := rfc1123SubdomainName(in)
				if got != want {
					t.Fatalf("baseline sanitization drift on zone=%q:\n  sprig = %q\n  go    = %q", in, got, want)
				}
			})
		}
	})

	// Override branch: resourcePool set on InstanceClass → template renders sanitized
	// "<zone>-<ng>", hook constructs the same via dzNameOverride.
	t.Run("override_with_resourcePool", func(t *testing.T) {
		ngNames := []string{
			"worker-fast",
			"Worker Fast",
			"Prod.Sys",
			"system_a",
		}
		for _, zone := range cases {
			for _, ng := range ngNames {
				t.Run(zone+"|"+ng, func(t *testing.T) {
					var buf bytes.Buffer
					ctx := map[string]any{
						"zone": zone,
						"instanceClass": map[string]any{
							"resourcePool": "/DC/host/cl/Resources/prod",
						},
						"nodeGroup": map[string]any{"name": ng},
					}
					if err := tmpl.Execute(&buf, ctx); err != nil {
						t.Fatalf("execute override sprig on zone=%q ng=%q: %v", zone, ng, err)
					}
					got := buf.String()
					want := dzNameOverride(zone, ng)
					if got != want {
						t.Fatalf("override sanitization drift on zone=%q ng=%q:\n  sprig = %q\n  go    = %q", zone, ng, got, want)
					}
				})
			}
		}
	})
}

// readSprigFailureDomainExpr loads template.yaml, yaml-parses it, and returns the raw
// string sitting at machineDeployment.additionalFields.failureDomain. It intentionally
// does not use the node-controller Contract parser — we want the literal that the
// template author put in the file, before any pre-processing.
func readSprigFailureDomainExpr(t *testing.T) string {
	t.Helper()
	// Test file sits in hooks/, template.yaml sits in ../capi/.
	path := filepath.Join("..", "capi", "template.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc struct {
		MachineDeployment struct {
			AdditionalFields map[string]string `json:"additionalFields"`
		} `json:"machineDeployment"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	expr, ok := doc.MachineDeployment.AdditionalFields["failureDomain"]
	if !ok || expr == "" {
		t.Fatalf("machineDeployment.additionalFields.failureDomain not found in %s", path)
	}
	return expr
}
