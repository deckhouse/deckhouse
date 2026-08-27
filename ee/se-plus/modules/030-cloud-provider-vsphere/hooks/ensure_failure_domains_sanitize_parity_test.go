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
// The two must match because CAPV's generateOverrideFunc looks up
// VSphereDeploymentZone by Machine.Spec.FailureDomain. This hook writes the DZ name
// through rfc1123SubdomainName; the template writes Machine.Spec.FailureDomain through
// the sprig expression. A silent divergence causes CAPV's per-zone datastore /
// resourcePool override to no-op — VMs clone with only the baseline topology from the
// template and end up on the wrong datastore, invisibly from kubectl.
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

	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, map[string]any{"zone": in}); err != nil {
				t.Fatalf("execute sprig sanitization on %q: %v", in, err)
			}
			got := buf.String()
			want := rfc1123SubdomainName(in)
			if got != want {
				t.Fatalf("sanitization drift for %q:\n  sprig (template.yaml) = %q\n  go    (hook)          = %q",
					in, got, want)
			}
		})
	}
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
