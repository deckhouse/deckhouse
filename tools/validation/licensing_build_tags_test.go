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
	"strings"
	"testing"
)

func TestEditionOf(t *testing.T) {
	for name, want := range map[string]string{
		"pkg/licensing_ce.go":       "ce",
		"pkg/licensing_ce_test.go":  "ce",
		"pkg/licensing_ee.go":       "ee",
		"pkg/licensing_ee_test.go":  "ee",
		"pkg/licensing_fe.go":       "fe",
		"pkg/licensing_fe_test.go":  "fe",
		"pkg/licensing.go":          "",
		"pkg/vendor_keys_dev.go":    "",
		"pkg/coffee.go":             "",
		"pkg/ee.go":                 "",
		"pkg/licensing_ee.yaml":     "",
		"pkg/licensing_ee_linux.go": "",
	} {
		if got := editionOf(name); got != want {
			t.Errorf("editionOf(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestCheckLicensingBuildTag(t *testing.T) {
	const header = "// Copyright 2026 Flant JSC\n//\n// Licensed under the Apache License, Version 2.0.\n\n"

	cases := []struct {
		name    string
		file    string
		src     string
		checked bool
		valid   bool
	}{
		// The name claims an edition: the build tag has to be that edition's.
		{"CE with its tag", "pkg/licensing_ce.go", "//go:build nolicensing\n\npackage pkg\n", true, true},
		{"CE test after the license header", "pkg/licensing_ce_test.go", header + "//go:build nolicensing\n\npackage pkg\n", true, true},
		{"EE with its tag", "pkg/licensing_ee.go", header + "//go:build !nolicensing\n\npackage pkg\n", true, true},
		{"EE test with its tag", "pkg/licensing_ee_test.go", "//go:build !nolicensing\n\npackage pkg\n", true, true},
		{"EE kept out of FE", "pkg/licensing_ee.go", "//go:build !nolicensing && !licensing_optin\n\npackage pkg\n", true, true},
		{"EE kept out of FE, operands swapped", "pkg/licensing_ee_test.go", "//go:build !licensing_optin && !nolicensing\n\npackage pkg\n", true, true},
		{"FE before a block license", "pkg/licensing_fe.go", "//go:build licensing_optin && !nolicensing\n\n/*\nCopyright 2026 Flant JSC\n*/\n\npackage pkg\n", true, true},
		{"FE test with the operands swapped", "pkg/licensing_fe_test.go", "//go:build !nolicensing && licensing_optin\n\npackage pkg\n", true, true},

		{"CE without a tag", "pkg/licensing_ce.go", header + "package pkg\n", true, false},
		{"EE without a tag", "pkg/licensing_ee_test.go", "package pkg\n", true, false},
		{"FE without a tag", "pkg/licensing_fe.go", "package pkg\n", true, false},
		{"EE built in CE", "pkg/licensing_ee.go", "//go:build nolicensing\n\npackage pkg\n", true, false},
		{"EE built in FE alone", "pkg/licensing_ee.go", "//go:build licensing_optin && !nolicensing\n\npackage pkg\n", true, false},
		{"FE with the EE tag", "pkg/licensing_fe.go", "//go:build !nolicensing\n\npackage pkg\n", true, false},
		{"CE with an unrelated tag", "pkg/licensing_ce.go", "//go:build linux\n\npackage pkg\n", true, false},
		{"tag in a block comment", "pkg/licensing_ce.go", "/*\n//go:build nolicensing\n*/\n\npackage pkg\n", true, false},
		{"tag below the package clause", "pkg/licensing_ce.go", "package pkg\n\n//go:build nolicensing\n", true, false},
		{"two tags", "pkg/licensing_ce.go", "//go:build nolicensing\n//go:build nolicensing\n\npackage pkg\n", true, false},
		{"not Go", "pkg/licensing_ce.go", "//go:build nolicensing\n\nnot a package clause\n", true, false},

		// The build tag selects editions: the name has to claim that edition.
		{"plain file with the EE tag", "pkg/licensing.go", header + "//go:build !nolicensing && !licensing_optin\n\npackage pkg\n", true, false},
		{"plain test with the CE tag", "pkg/licensing_test.go", "//go:build nolicensing\n\npackage pkg\n", true, false},
		{"plain file with the FE tag", "pkg/licensing.go", "//go:build licensing_optin && !nolicensing\n\npackage pkg\n", true, false},
		{"plain file with a tag of no edition", "pkg/licensing.go", "//go:build nolicensing || licensing_optin\n\npackage pkg\n", true, false},
		{"plain file with a broken EE tag", "pkg/licensing.go", header + "//go:build !licensing_optin && !nolicensing &&\n\npackage pkg\n", true, false},
		{"plain file with two tags, one on the licensing tags", "pkg/licensing.go", "//go:build linux\n//go:build nolicensing\n\npackage pkg\n", true, false},
		{"EE with a broken tag", "pkg/licensing_ee.go", "//go:build !nolicensing &&\n\npackage pkg\n", true, false},

		// Neither: not checked.
		{"plain file without a tag", "pkg/licensing.go", "package pkg\n", false, true},
		{"plain file with an unrelated tag", "pkg/vendor_keys_dev.go", "//go:build licensing_testkey\n\npackage pkg\n", false, true},
		{"plain file with a broken unrelated tag", "pkg/linux.go", "//go:build linux &&\n\npackage pkg\n", false, true},
		{"plain file that is not Go", "pkg/template.go", "not a package clause\n", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg, checked := checkLicensingBuildTag(tc.file, []byte(tc.src))
			if checked != tc.checked {
				t.Fatalf("checked = %v, want %v", checked, tc.checked)
			}
			if checked && msg.IsError() == tc.valid {
				t.Errorf("valid = %v, got:\n%s", tc.valid, msg.Format())
			}
		})
	}
}

func TestEditionFileName(t *testing.T) {
	for _, tc := range []struct{ file, edition, want string }{
		{"pkg/licensing.go", "ee", "pkg/licensing_ee.go"},
		{"pkg/licensing_test.go", "ce", "pkg/licensing_ce_test.go"},
		{"pkg/gate.go", "fe", "pkg/gate_fe.go"},
	} {
		if got := editionFileName(tc.file, tc.edition); got != tc.want {
			t.Errorf("editionFileName(%q, %q) = %q, want %q", tc.file, tc.edition, got, tc.want)
		}
	}
}

// A pure rename has no ---/+++ lines; the file is still found by the header.
func TestParseDiffOutputKeepsTheHeaderFileName(t *testing.T) {
	diff := `diff --git a/pkg/registration.go b/pkg/registration_ee.go
similarity index 100%
rename from pkg/registration.go
rename to pkg/registration_ee.go
diff --git a/pkg/other.go b/pkg/other.go
index 1111111..2222222 100644
--- a/pkg/other.go
+++ b/pkg/other.go
@@ -1 +1 @@
-package pkg
+package pkg // changed
`
	info, err := ParseDiffOutput(strings.NewReader(diff))
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Files) != 2 {
		t.Fatalf("got %d files, want 2", len(info.Files))
	}

	renamed := info.Files[0]
	if renamed.NewFileName != "" || renamed.HeaderFileName != "pkg/registration_ee.go" || renamed.HasContent() {
		t.Errorf("rename: new %q, header %q, lines %d", renamed.NewFileName, renamed.HeaderFileName, len(renamed.Lines))
	}
	changed := info.Files[1]
	if changed.NewFileName != "pkg/other.go" || changed.HeaderFileName != "pkg/other.go" {
		t.Errorf("change: new %q, header %q", changed.NewFileName, changed.HeaderFileName)
	}
}
