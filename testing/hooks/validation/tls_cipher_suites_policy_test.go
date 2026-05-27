//go:build validation
// +build validation

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

package validation

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestValidationTLSCipherSuitesPolicy enforces the deckhouse TLS standard
// described in go_lib/hooks/tls_certificate/README.md ("TLS profiles").
//
// For every place that explicitly lists TLS cipher suites — Helm/werf
// arguments (`--tls-cipher-suites=…`), kubelet configuration
// (`tlsCipherSuites: […]`) and Go `tls.Config{CipherSuites: …}` — the
// list must contain no entry that is forbidden by the standard:
//
//   - TLS_RSA_WITH_*   (RSA key exchange, no Perfect Forward Secrecy);
//   - *_CBC_*          (Lucky13 family);
//   - *_SHA            (legacy SHA1; allowed suffixes are _SHA256/_SHA384);
//   - TLS_GOSTR341112_* / KUZNYECHIK / MAGMA
//     (not implemented in upstream Go — would silently never match in the
//     handshake; their presence creates a false sense of GOST support).
//
// The check is intentionally a deny-list, not a whitelist: the standard
// allows kube-apiserver to keep historical aliases such as
// TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305 (deprecated synonym of the
// _SHA256 variant) as long as no forbidden entry sneaks in.
func TestValidationTLSCipherSuitesPolicy(t *testing.T) {
	const repoRoot = "/deckhouse"

	roots := tlsPolicyScanRoots(repoRoot)
	require.NotEmpty(t, roots, "no scan roots resolved; repo layout changed?")

	violations := scanForbiddenCipherSuites(t, roots)
	if len(violations) == 0 {
		return
	}

	sort.Strings(violations)
	t.Fatalf(
		"Found %d violation(s) of the deckhouse TLS cipher-suites standard\n"+
			"(see go_lib/hooks/tls_certificate/README.md, section 'TLS profiles').\n"+
			"Forbidden patterns: TLS_RSA_WITH_*, *_CBC_*, *_SHA (without "+
			"SHA256/SHA384), TLS_GOSTR341112_*, KUZNYECHIK, MAGMA.\n\n"+
			"Violations:\n  %s",
		len(violations), strings.Join(violations, "\n  "),
	)
}

// TestValidationCategoryAWebhookServersUseTLS13 enforces that every
// admission/conversion webhook (and extension API server) shipped by
// deckhouse pins MinVersion to TLS 1.3. The check is textual on purpose:
// each webhook lives in its own go.mod-module so we cannot reuse a
// helper symbol from go_lib; we just verify the literal
// `tls.VersionTLS13` is mentioned next to the webhook listener.
func TestValidationCategoryAWebhookServersUseTLS13(t *testing.T) {
	const repoRoot = "/deckhouse"

	// Each (sourceFile, hint) pair: a relative path expected to mention
	// tls.VersionTLS13 and a short description for the error message.
	// The list mirrors the audit in
	// go_lib/hooks/tls_certificate/README.md, section "TLS profiles".
	wantTLS13 := []categoryASource{
		// admission / conversion webhooks
		{"modules/002-deckhouse/images/webhook-handler/operator/cmd/main.go", "deckhouse webhook-handler"},
		{"ee/be/modules/140-user-authz/images/webhook/src/internal/web/server.go", "user-authz webhook"},
		{"modules/042-kube-dns/images/sts-pods-hosts-appender-webhook/src/main.go", "sts-pods-hosts-appender mutating webhook"},
		{"modules/160-multitenancy-manager/images/multitenancy-manager/src/cmd/main.go", "multitenancy-manager webhooks"},
		{"modules/040-node-manager/images/node-controller/src/cmd/main.go", "node-controller webhooks"},
		{"modules/040-node-manager/images/caps-controller-manager/src/cmd/main.go", "caps-controller-manager webhooks"},
		{"modules/030-cloud-provider-dvp/images/capdvp-controller-manager/src/cmd/main.go", "capdvp-controller-manager webhook"},
		{"ee/modules/030-cloud-provider-vcd/images/infra-controller-manager/src/cmd/main.go", "vcd infra-controller-manager webhook"},
		{"ee/se-plus/modules/030-cloud-provider-zvirt/images/capz-controller-manager/src/cmd/main.go", "capz-controller-manager webhook"},
		{"ee/se-plus/modules/021-cni-cilium/images/egress-gateway-agent/src/cmd/main.go", "egress-gateway-agent webhook"},
	}

	var failures []string
	for _, src := range wantTLS13 {
		abs := filepath.Join(repoRoot, src.relPath)
		body, err := os.ReadFile(abs)
		if err != nil {
			failures = append(failures, src.relPath+": cannot read: "+err.Error())
			continue
		}
		if !strings.Contains(string(body), "tls.VersionTLS13") {
			failures = append(failures, src.relPath+": "+src.description+
				" must pin tls.Config.MinVersion to tls.VersionTLS13 "+
				"(see go_lib/hooks/tls_certificate/README.md, Category A)")
		}
	}

	if len(failures) > 0 {
		sort.Strings(failures)
		t.Fatalf("Category A TLS profile not applied in %d file(s):\n  %s",
			len(failures), strings.Join(failures, "\n  "))
	}
}

type categoryASource struct {
	relPath     string
	description string
}

// tlsPolicyScanRoots returns the directories under which TLS cipher-
// suite configuration is expected to live. We intentionally limit the
// walk to the dirs that ship into production clusters: modules/, ee/,
// candi/, helm_lib/, deckhouse-controller/. Hot paths like docs/,
// CHANGELOG/ and the validation test itself are excluded.
func tlsPolicyScanRoots(repoRoot string) []string {
	candidates := []string{
		filepath.Join(repoRoot, "modules"),
		filepath.Join(repoRoot, "ee"),
		filepath.Join(repoRoot, "candi"),
		filepath.Join(repoRoot, "helm_lib"),
		filepath.Join(repoRoot, "deckhouse-controller"),
		filepath.Join(repoRoot, "global-hooks"),
		filepath.Join(repoRoot, "dhctl"),
	}
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			out = append(out, c)
		}
	}
	return out
}

// tlsCipherLineRE matches lines that contain a cipher-suite list. We
// match either the k8s component-base flag (`--tls-cipher-suites=…` or
// `"--tls-cipher-suites", "…"`), the kubelet config key
// (`tlsCipherSuites: [...]`) or the Go field name (`CipherSuites:`).
// The match is per-line on purpose: we do not parse YAML/Go, we just
// look at the textual presence of forbidden suite names.
var tlsCipherLineRE = regexp.MustCompile(
	`(?i)(--tls-cipher-suites|tlsciphersuites\s*:|ciphersuites\s*:|cipher-suites)`)

// forbiddenCipherREs are evaluated against the raw line. They cover the
// four classes called out by the standard.
var forbiddenCipherREs = []struct {
	name string
	re   *regexp.Regexp
}{
	{"TLS_RSA_WITH_* (no PFS)", regexp.MustCompile(`\bTLS_RSA_WITH_[A-Z0-9_]+\b`)},
	{"*_CBC_* (Lucky13)", regexp.MustCompile(`\bTLS_[A-Z0-9_]*_CBC_[A-Z0-9_]+\b`)},
	{"*_SHA without SHA256/SHA384 (legacy SHA1)", regexp.MustCompile(`\bTLS_[A-Z0-9_]+_SHA\b`)},
	{"TLS_GOSTR341112_* / KUZNYECHIK / MAGMA (not in upstream Go)",
		regexp.MustCompile(`\b(TLS_GOSTR341112_[A-Z0-9_]+|KUZNYECHIK|MAGMA)\b`)},
}

// tlsPolicySkipFiles lists paths that legitimately mention forbidden
// suite names (e.g. as test fixtures, denial assertions or release
// notes). Paths are relative to the repository root.
var tlsPolicySkipFiles = map[string]string{
	// The helper test file deliberately uses TLS_RSA_WITH_AES_128_CBC_SHA
	// as input to verify that ApplyServerCategoryA() clears CipherSuites.
	"go_lib/hooks/tls_certificate/server_config_test.go": "negative-test fixture for the helper",
}

// tlsPolicyAllowedExtensions limits the walk to files where TLS cipher-
// suite lists realistically appear.
var tlsPolicyAllowedExtensions = map[string]struct{}{
	".yaml": {},
	".yml":  {},
	".tpl":  {},
	".sh":   {},
	".go":   {},
}

// tlsPolicySkipDirNames are walked over but never descended into.
var tlsPolicySkipDirNames = map[string]struct{}{
	"vendor":         {},
	"node_modules":   {},
	"testdata":       {},
	"test-data":      {},
	"crds":           {},
	"internal_tls":   {},
	"openapi":        {},
	"openapi_v3":     {},
	".git":           {},
	".helm":          {},
	".terraform":     {},
	"release-notes":  {},
	"docs":           {},
	"CHANGELOG":      {},
	"hack":           {},
	"e2e":            {},
	"benchmarks":     {},
	"helm_lib_tests": {},
}

func scanForbiddenCipherSuites(t *testing.T, roots []string) []string {
	t.Helper()

	var violations []string
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if _, skip := tlsPolicySkipDirNames[d.Name()]; skip {
					return filepath.SkipDir
				}
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if _, ok := tlsPolicyAllowedExtensions[ext]; !ok {
				return nil
			}
			if rel, ok := tlsPolicyRelToDeckhouse(path); ok {
				if _, skip := tlsPolicySkipFiles[rel]; skip {
					return nil
				}
			}
			fileViolations := scanFileForForbiddenSuites(path)
			violations = append(violations, fileViolations...)
			return nil
		})
	}
	return violations
}

// tlsPolicyRelToDeckhouse converts an absolute path under /deckhouse to
// its repo-relative form. Returns ("", false) for paths outside the
// repository.
func tlsPolicyRelToDeckhouse(abs string) (string, bool) {
	const root = "/deckhouse/"
	if !strings.HasPrefix(abs, root) {
		return "", false
	}
	return strings.TrimPrefix(abs, root), true
}

func scanFileForForbiddenSuites(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	rel, _ := tlsPolicyRelToDeckhouse(path)
	if rel == "" {
		rel = path
	}

	var violations []string
	scanner := bufio.NewScanner(f)
	// Allow long single-line flags like `--tls-cipher-suites=a,b,c,d,…`.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineno := 0
	for scanner.Scan() {
		lineno++
		line := scanner.Text()
		if !tlsCipherLineRE.MatchString(line) {
			continue
		}
		for _, rule := range forbiddenCipherREs {
			if m := rule.re.FindString(line); m != "" {
				violations = append(violations,
					rel+":"+strconv.Itoa(lineno)+": forbidden "+rule.name+" — "+m)
			}
		}
	}
	return violations
}
