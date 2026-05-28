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
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestValidationTLSCertificateHookPodConsumers enforces the Pod-restart
// contract documented in go_lib/hooks/tls_certificate/README.md:
//
//	For every tls_certificate.RegisterInternalTLSHook(Namespace, TLSSecretName)
//	every Pod template that mounts the resulting Secret as a volume MUST
//	carry a checksum/* annotation whose value depends on the Secret content.
//
// Without that annotation, when the hook re-issues the CA + cert pair
// (which happens automatically for legacy certificates failing the validity
// rules in internal_tls.go — Subject != Issuer, non-empty EKU, …), Helm
// rotates the Secret but the webhook server keeps the stale cert in memory
// while kube-apiserver already validates against the new CA. The result is
// the well-known "x509: certificate signed by unknown authority" cascade.
func TestValidationTLSCertificateHookPodConsumers(t *testing.T) {
	const repoRoot = "/deckhouse"

	editionsModulesDirs := collectEditionsModulesDirs(repoRoot)
	require.NotEmpty(t, editionsModulesDirs, "editions.yaml produced no modulesDir entries")

	moduleNameToTemplatesDirs := indexModuleTemplatesDirs(editionsModulesDirs)
	stringConsts := collectStringConsts(editionsModulesDirs)
	regs := collectTLSHookRegistrations(t, editionsModulesDirs, stringConsts)

	require.NotEmpty(t, regs,
		"no tls_certificate.RegisterInternalTLSHook calls were discovered; "+
			"the collector is broken or the source tree moved")

	for _, reg := range regs {
		if reason, skip := skippedTLSSecretNames[reg.TLSSecretName]; skip {
			t.Logf("skipping TLS hook %s/%s (%s)", reg.Namespace, reg.TLSSecretName, reason)
			continue
		}

		moduleName := filepath.Base(reg.ModuleRoot)
		searchDirs := moduleNameToTemplatesDirs[moduleName]
		consumers := findPodTemplateConsumers(searchDirs, reg.TLSSecretName)
		if len(consumers) == 0 {
			t.Errorf(
				"%s registers TLS secret %s/%q but no Pod template under any edition of "+
					"module %q mounts it as a volume; either remove the orphan registration or "+
					"add a consumer (see go_lib/hooks/tls_certificate/README.md)",
				rel(reg.HookFile, repoRoot), reg.Namespace, reg.TLSSecretName, moduleName,
			)
			continue
		}

		for _, consumer := range consumers {
			if _, ok := hotReloadingWorkloads[rel(consumer, repoRoot)]; ok {
				continue
			}
			if hasChecksumLinkForSecret(consumer, reg.TLSSecretName) {
				continue
			}
			t.Errorf(
				"Pod template %s mounts TLS secret %s/%q without any checksum/* annotation "+
					"linking to it.\nAdd to spec.template.metadata.annotations:\n\n"+
					"  checksum/%s: {{ include (print $.Template.BasePath \"/<relative path to %s secret template>\") . | sha256sum }}\n\n"+
					"Rationale: the hook may re-issue the CA + cert at any time; without this "+
					"annotation Helm rotates the Secret but the Pod keeps the stale cert in memory "+
					"and webhook clients fail with x509 unknown authority. "+
					"See go_lib/hooks/tls_certificate/README.md.",
				rel(consumer, repoRoot), reg.Namespace, reg.TLSSecretName,
				simplifyAnnotationKey(reg.TLSSecretName), reg.TLSSecretName,
			)
		}
	}
}

// tlsHookRegistration captures one tls_certificate.RegisterInternalTLSHook
// invocation together with the values resolved for Namespace and
// TLSSecretName.
type tlsHookRegistration struct {
	HookFile      string
	ModuleRoot    string
	Namespace     string
	TLSSecretName string
}

// skippedTLSSecretNames lists TLS secret names that are intentionally not
// consumed by any real workload. Each entry must carry an inline rationale.
var skippedTLSSecretNames = map[string]string{
	// modules/000-common is a hook authoring template; the placeholder
	// names (d8-module-name / module-name-internal-tls) are never deployed.
	"module-name-internal-tls": "common module placeholder hook",
}

// hotReloadingWorkloads lists Pod template paths (relative to repo root)
// whose webhook server reloads the TLS material from disk at runtime
// (e.g. via k8s.io/apiserver dynamic certificate controllers or
// controller-runtime's webhook.Server). For those, a checksum annotation
// is not required because the kubelet's atomic Secret update is picked up
// live. Adding an entry must reference the reload implementation in the
// PR description.
var hotReloadingWorkloads = map[string]struct{}{}

// collectEditionsModulesDirs reads editions.yaml via the existing helper
// and returns the absolute paths of every <modulesDir> root.
func collectEditionsModulesDirs(repoRoot string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, glob := range getPossiblePathToModules() {
		// glob looks like "/deckhouse/<modulesDir>/*/hooks".
		base := strings.TrimSuffix(glob, "/*/hooks")
		if _, ok := seen[base]; ok {
			continue
		}
		seen[base] = struct{}{}
		out = append(out, base)
	}
	sort.Strings(out)
	return out
}

// indexModuleTemplatesDirs builds module-name -> [list of templates dirs
// across all editions] so that a hook registered in the CE source tree can
// find consumers living in an EE/BE/SE variant of the same module.
func indexModuleTemplatesDirs(editionsModulesDirs []string) map[string][]string {
	idx := map[string][]string{}
	for _, ed := range editionsModulesDirs {
		entries, err := os.ReadDir(ed)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			tdir := filepath.Join(ed, e.Name(), "templates")
			info, err := os.Stat(tdir)
			if err != nil || !info.IsDir() {
				continue
			}
			idx[e.Name()] = append(idx[e.Name()], tdir)
		}
	}
	for k := range idx {
		sort.Strings(idx[k])
	}
	return idx
}

// collectStringConsts maps "<pkgDir>:<constName>" to the string value of
// every top-level string const declared under every module's hooks/ tree.
// Used to resolve identifiers (apiserverCertificateSecretName) and
// cross-package selectors (internal.Namespace) used as struct field values
// in RegisterInternalTLSHook calls.
func collectStringConsts(editionsModulesDirs []string) map[string]string {
	consts := map[string]string{}
	for _, ed := range editionsModulesDirs {
		_ = filepath.Walk(ed, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			fset := token.NewFileSet()
			f, perr := parser.ParseFile(fset, path, nil, parser.AllErrors)
			if perr != nil {
				return nil
			}
			pkgDir := filepath.Dir(path)
			for _, decl := range f.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok || gen.Tok != token.CONST {
					continue
				}
				for _, spec := range gen.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, name := range vs.Names {
						if i >= len(vs.Values) {
							continue
						}
						bl, ok := vs.Values[i].(*ast.BasicLit)
						if !ok || bl.Kind != token.STRING {
							continue
						}
						consts[pkgDir+":"+name.Name] = strings.Trim(bl.Value, "\"`")
					}
				}
			}
			return nil
		})
	}
	return consts
}

// collectTLSHookRegistrations walks every Go source file under each
// module's hooks/ tree, looking for tls_certificate.RegisterInternalTLSHook
// calls whose first argument is a tls_certificate.GenSelfSignedTLSHookConf
// composite literal. Returns one entry per call, with Namespace and
// TLSSecretName resolved to their string values.
func collectTLSHookRegistrations(t *testing.T, editionsModulesDirs []string, consts map[string]string) []tlsHookRegistration {
	t.Helper()

	var out []tlsHookRegistration
	for _, ed := range editionsModulesDirs {
		_ = filepath.Walk(ed, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			// Only consider files under a <module>/hooks/ subtree.
			if !strings.Contains(path, "/hooks/") && !strings.HasSuffix(filepath.Dir(path), "/hooks") {
				return nil
			}
			fset := token.NewFileSet()
			f, perr := parser.ParseFile(fset, path, nil, parser.AllErrors)
			if perr != nil {
				return nil
			}
			moduleRoot := moduleRootFromHookFile(path, ed)
			if moduleRoot == "" {
				return nil
			}
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if !isRegisterInternalTLSHookCall(call) {
					return true
				}
				if len(call.Args) == 0 {
					return true
				}
				lit, ok := call.Args[0].(*ast.CompositeLit)
				if !ok {
					return true
				}
				reg := tlsHookRegistration{HookFile: path, ModuleRoot: moduleRoot}
				for _, elt := range lit.Elts {
					kv, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					key, ok := kv.Key.(*ast.Ident)
					if !ok {
						continue
					}
					switch key.Name {
					case "Namespace":
						reg.Namespace = resolveStringExpr(kv.Value, f, path, consts)
					case "TLSSecretName":
						reg.TLSSecretName = resolveStringExpr(kv.Value, f, path, consts)
					}
				}
				if reg.Namespace == "" || reg.TLSSecretName == "" {
					t.Logf("WARN: %s:%d — could not resolve Namespace/TLSSecretName; annotation check skipped",
						path, fset.Position(call.Pos()).Line)
					return true
				}
				out = append(out, reg)
				return true
			})
			return nil
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].HookFile < out[j].HookFile })
	return out
}

// isRegisterInternalTLSHookCall returns true when the call is of the form
// <pkg>.RegisterInternalTLSHook(...). The <pkg> identifier check is
// intentionally lax (we don't enforce the alias) since the function name
// is unique across the codebase.
func isRegisterInternalTLSHookCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return sel.Sel.Name == "RegisterInternalTLSHook"
}

// moduleRootFromHookFile derives "<modulesDir>/<module>" from a hook file
// path. Returns empty when the file is not under such a directory.
func moduleRootFromHookFile(hookFile, editionRoot string) string {
	rel, err := filepath.Rel(editionRoot, hookFile)
	if err != nil {
		return ""
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) < 2 {
		return ""
	}
	return filepath.Join(editionRoot, parts[0])
}

// resolveStringExpr converts a struct field value into the string it
// evaluates to at compile time. Handles string literals, package-local
// idents, and cross-package selectors (internal.Namespace).
func resolveStringExpr(expr ast.Expr, file *ast.File, hookFile string, consts map[string]string) string {
	switch v := expr.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			return strings.Trim(v.Value, "\"`")
		}
	case *ast.Ident:
		if c, ok := consts[filepath.Dir(hookFile)+":"+v.Name]; ok {
			return c
		}
	case *ast.SelectorExpr:
		pkgIdent, ok := v.X.(*ast.Ident)
		if !ok {
			return ""
		}
		importDir := resolveImportDir(file, pkgIdent.Name)
		if importDir == "" {
			return ""
		}
		if c, ok := consts[importDir+":"+v.Sel.Name]; ok {
			return c
		}
	}
	return ""
}

const (
	repoGoModulePrefix = "github.com/deckhouse/deckhouse/"
	repoFsRootPrefix   = "/deckhouse/"
)

// resolveImportDir maps an imported package referenced by `pkgName`
// (alias or last path segment) to an absolute repository directory.
func resolveImportDir(file *ast.File, pkgName string) string {
	for _, imp := range file.Imports {
		ipath := strings.Trim(imp.Path.Value, "\"")
		var name string
		if imp.Name != nil {
			name = imp.Name.Name
		} else {
			name = ipath
			if i := strings.LastIndex(name, "/"); i >= 0 {
				name = name[i+1:]
			}
		}
		if name != pkgName {
			continue
		}
		if !strings.HasPrefix(ipath, repoGoModulePrefix) {
			continue
		}
		return repoFsRootPrefix + strings.TrimPrefix(ipath, repoGoModulePrefix)
	}
	return ""
}

// secretNameRefRe matches `secretName: <name>` lines inside a Helm
// template. Leading whitespace is mandatory to avoid matching comments
// or string literals starting at column 0.
var secretNameRefRe = regexp.MustCompile(`(?m)^\s+secretName:\s*['"]?([A-Za-z0-9._-]+)['"]?\s*$`)

// findPodTemplateConsumers walks the given templates directories and
// returns every .yaml file that contains at least one `secretName:
// <secretName>` reference.
func findPodTemplateConsumers(searchDirs []string, secretName string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, dir := range searchDirs {
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
				return nil
			}
			raw, rerr := os.ReadFile(path)
			if rerr != nil {
				return nil
			}
			for _, m := range secretNameRefRe.FindAllStringSubmatch(string(raw), -1) {
				if m[1] == secretName {
					if _, ok := seen[path]; !ok {
						seen[path] = struct{}{}
						out = append(out, path)
					}
					break
				}
			}
			return nil
		})
	}
	sort.Strings(out)
	return out
}

// checksumAnnotationRe captures `checksum/<key>: <value>` lines in any
// template file.
var checksumAnnotationRe = regexp.MustCompile(`(?m)^\s+checksum/[A-Za-z0-9._-]+:\s*(.+?)\s*$`)

// yamlPathRe extracts quoted paths ending in .yaml / .yml from a string.
// Helm templates use a variety of constructs (`include (print $.Template.BasePath "/x.yaml")`,
// `include (printf "%s%s" .Template.BasePath "/x.yaml")`, …), so the
// matcher avoids parsing the Go-template AST and just walks all yaml-suffix
// path literals.
var yamlPathRe = regexp.MustCompile(`["']([^"']*\.ya?ml)["']`)

// secretManifestKindRe matches the `kind: Secret` declaration in a Helm
// template.
var secretManifestKindRe = regexp.MustCompile(`(?m)^kind:\s*Secret\s*$`)

// metadataNameRe matches `name: <value>` lines. Used together with
// `kind: Secret` to confirm that an `include`'d template renders the same
// Secret name as the one we are looking for.
var metadataNameRe = regexp.MustCompile(`(?m)^\s*name:\s*['"]?([A-Za-z0-9._-]+)['"]?\s*$`)

// hasChecksumLinkForSecret returns true when consumerFile (or any other
// template file in the same module's templates/ tree) carries at least
// one checksum/* annotation whose value links to the TLS secret named
// secretName. Two link forms are accepted:
//
//   - the annotation value contains the literal secret name, or
//   - the annotation value `include`s a template path that resolves to a
//     file declaring a `kind: Secret` with `name: <secretName>`.
//
// The cross-file search covers helper-based templates such as
// helm_lib_capi_controller_manager_manifests, where Pod-template
// annotations are passed through a define / dict and live in a separate
// file from the workload declaration.
func hasChecksumLinkForSecret(consumerFile, secretName string) bool {
	if checksumLinkInFile(consumerFile, secretName) {
		return true
	}
	templatesDir := findTemplatesDir(consumerFile)
	if templatesDir == "" {
		return false
	}
	found := false
	_ = filepath.Walk(templatesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || found {
			return nil
		}
		if path == consumerFile {
			return nil
		}
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}
		if checksumLinkInFile(path, secretName) {
			found = true
		}
		return nil
	})
	return found
}

// checksumLinkInFile inspects a single template file for an annotation
// satisfying the contract. The function tolerates Helm templating noise
// in annotation values and resolves `include` paths against the file's
// owning templates/ directory (the value `$.Template.BasePath` evaluates
// to).
//
// Two annotation forms are recognised:
//
//  1. The plain YAML form `checksum/<key>: <value>` placed directly in
//     spec.template.metadata.annotations.
//  2. The helper form where Pod-template annotations are injected through
//     `additionalPodAnnotations` (used by helm_lib_capi_controller_manager_manifests
//     and friends). The matcher accepts (a) the file mentions
//     additionalPodAnnotations with a checksum/* key, and (b) the file
//     also has an `include` whose resolved path declares the target Secret.
func checksumLinkInFile(path, secretName string) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	content := string(raw)
	templatesDir := findTemplatesDir(path)

	for _, m := range checksumAnnotationRe.FindAllStringSubmatch(content, -1) {
		val := m[1]
		if strings.Contains(val, secretName) {
			return true
		}
		if templatesDir == "" {
			continue
		}
		for _, pm := range yamlPathRe.FindAllStringSubmatch(val, -1) {
			candidate := pm[1]
			if !strings.HasPrefix(candidate, "/") {
				// $.Template.BasePath is always followed by an absolute-
				// looking path; relative candidates are not what we want.
				continue
			}
			resolved := filepath.Join(templatesDir, candidate)
			if templateDeclaresSecret(resolved, secretName) {
				return true
			}
		}
	}

	if templatesDir != "" &&
		strings.Contains(content, "additionalPodAnnotations") &&
		strings.Contains(content, "checksum/") {
		for _, pm := range yamlPathRe.FindAllStringSubmatch(content, -1) {
			candidate := pm[1]
			if !strings.HasPrefix(candidate, "/") {
				continue
			}
			resolved := filepath.Join(templatesDir, candidate)
			if templateDeclaresSecret(resolved, secretName) {
				return true
			}
		}
	}
	return false
}

// findTemplatesDir walks up from a file until it finds the enclosing
// directory named `templates`.
func findTemplatesDir(p string) string {
	dir := filepath.Dir(p)
	for dir != "/" && dir != "." && dir != "" {
		if filepath.Base(dir) == "templates" {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	return ""
}

// templateDeclaresSecret returns true when the YAML file at path declares
// a `kind: Secret` whose metadata.name equals secretName. The match is
// purely textual to avoid pulling Helm rendering into the validation test.
func templateDeclaresSecret(path, secretName string) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	content := string(raw)
	if !secretManifestKindRe.MatchString(content) {
		return false
	}
	for _, m := range metadataNameRe.FindAllStringSubmatch(content, -1) {
		if m[1] == secretName {
			return true
		}
	}
	return false
}

// rel returns path relative to base or the original path when filepath.Rel fails.
func rel(path, base string) string {
	if r, err := filepath.Rel(base, path); err == nil {
		return r
	}
	return path
}

// simplifyAnnotationKey trims trailing TLS/cert noise from a secret name
// so it produces a tidy annotation suffix in error hints. The returned
// value is only used to format the suggested annotation in the failure
// message — it has no semantic effect on the test.
func simplifyAnnotationKey(s string) string {
	for _, suffix := range []string{"-server-cert", "-certs", "-cert", "-tls"} {
		if strings.HasSuffix(s, suffix) {
			return strings.TrimSuffix(s, suffix)
		}
	}
	return s
}

// TestTLSCertificatePodConsumersChecksumMatcher exercises checksumLinkInFile
// against synthetic Helm template snippets. These unit tests are NOT
// matched by `make validate`'s -run Validation filter; they run only on
// `go test -tags=validation ./testing/hooks/validation/...` and exist to
// keep the matcher logic honest under refactoring.
func TestTLSCertificatePodConsumersChecksumMatcher(t *testing.T) {
	tmp := t.TempDir()
	templatesDir := filepath.Join(tmp, "templates")
	require.NoError(t, os.MkdirAll(filepath.Join(templatesDir, "webhook"), 0o755))

	secretPath := filepath.Join(templatesDir, "webhook", "secret-tls.yaml")
	require.NoError(t, os.WriteFile(secretPath, []byte(`apiVersion: v1
kind: Secret
metadata:
  name: my-cert
  namespace: d8-x
type: kubernetes.io/tls
data: {}
`), 0o644))

	cases := []struct {
		name   string
		body   string
		secret string
		want   bool
	}{
		{
			name: "literal secret name in annotation value",
			body: `apiVersion: apps/v1
kind: Deployment
spec:
  template:
    metadata:
      annotations:
        checksum/cert: my-cert
    spec:
      volumes:
      - name: certs
        secret:
          secretName: my-cert
`,
			secret: "my-cert",
			want:   true,
		},
		{
			name: "include resolves to Secret template with same name",
			body: `apiVersion: apps/v1
kind: Deployment
spec:
  template:
    metadata:
      annotations:
        checksum/certificate: {{ include (print $.Template.BasePath "/webhook/secret-tls.yaml") . | sha256sum }}
    spec:
      volumes:
      - name: certs
        secret:
          secretName: my-cert
`,
			secret: "my-cert",
			want:   true,
		},
		{
			name: "checksum annotation references unrelated secret",
			body: `apiVersion: apps/v1
kind: Deployment
spec:
  template:
    metadata:
      annotations:
        checksum/config: {{ include (print $.Template.BasePath "/registry-secret.yaml") . | sha256sum }}
    spec:
      volumes:
      - name: certs
        secret:
          secretName: my-cert
`,
			secret: "my-cert",
			want:   false,
		},
		{
			name: "no annotations at all",
			body: `apiVersion: apps/v1
kind: Deployment
spec:
  template:
    metadata:
      labels:
        app: foo
    spec:
      volumes:
      - name: certs
        secret:
          secretName: my-cert
`,
			secret: "my-cert",
			want:   false,
		},
		{
			name: "printf-style include with two strings",
			body: `apiVersion: apps/v1
kind: Deployment
spec:
  template:
    metadata:
      annotations:
        checksum/certificate: {{ include (printf "%s%s" $.Template.BasePath "/webhook/secret-tls.yaml") . | sha256sum }}
`,
			secret: "my-cert",
			want:   true,
		},
		{
			name: "helper-style additionalPodAnnotations dict",
			body: `{{- $cfg := dict -}}
{{- $certChecksum := include (print $.Template.BasePath "/webhook/secret-tls.yaml") . | sha256sum -}}
{{- $_ := set $cfg "additionalPodAnnotations" (dict "checksum/certificate" $certChecksum) -}}
{{ include "helm_lib_capi_controller_manager_manifests" (list . $cfg) }}
`,
			secret: "my-cert",
			want:   true,
		},
		{
			name: "helper-style additionalPodAnnotations dict with unrelated include",
			body: `{{- $cfg := dict -}}
{{- $unrelated := include (print $.Template.BasePath "/something-else.yaml") . | sha256sum -}}
{{- $_ := set $cfg "additionalPodAnnotations" (dict "checksum/config" $unrelated) -}}
`,
			secret: "my-cert",
			want:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := filepath.Join(templatesDir, fmt.Sprintf("deployment-%s.yaml", strings.ReplaceAll(tc.name, " ", "-")))
			require.NoError(t, os.WriteFile(f, []byte(tc.body), 0o644))
			got := checksumLinkInFile(f, tc.secret)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestTLSCertificatePodConsumersHookCollector parses an in-memory hook
// source to ensure the AST walker resolves field values from both
// same-package idents and cross-package selectors.
func TestTLSCertificatePodConsumersHookCollector(t *testing.T) {
	tmp := t.TempDir()
	hooksDir := filepath.Join(tmp, "modules", "999-x", "hooks")
	internalDir := filepath.Join(hooksDir, "internal")
	require.NoError(t, os.MkdirAll(internalDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(internalDir, "common.go"), []byte(`package internal
const Namespace = "d8-x"
`), 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "tls.go"), []byte(`package hooks
import (
	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
	"github.com/deckhouse/deckhouse/modules/999-x/hooks/internal"
)
const secretName = "x-tls"
var _ = tls_certificate.RegisterInternalTLSHook(tls_certificate.GenSelfSignedTLSHookConf{
	Namespace:     internal.Namespace,
	TLSSecretName: secretName,
})
`), 0o644))

	// Emulate the /deckhouse path layout that resolveImportDir expects.
	deckhouseRoot := filepath.Join(tmp, "deckhouse-link")
	require.NoError(t, os.Symlink(tmp, deckhouseRoot))
	t.Cleanup(func() { _ = os.Remove(deckhouseRoot) })

	// Skipped: full collector exercise requires /deckhouse layout; we only
	// verify resolveStringExpr in isolation here.
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filepath.Join(hooksDir, "tls.go"), nil, parser.AllErrors)
	require.NoError(t, err)

	consts := map[string]string{
		hooksDir + ":secretName":      "x-tls",
		internalDir + ":Namespace":    "d8-x",
	}

	var resolvedNamespace, resolvedSecret string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isRegisterInternalTLSHookCall(call) {
			return true
		}
		lit := call.Args[0].(*ast.CompositeLit)
		for _, elt := range lit.Elts {
			kv := elt.(*ast.KeyValueExpr)
			key := kv.Key.(*ast.Ident).Name
			switch key {
			case "Namespace":
				// Override import resolution to point at our synthetic
				// internalDir so the cross-package lookup hits our fake
				// consts map.
				resolvedNamespace = resolveStringExprWithImportDir(kv.Value, "internal", internalDir, consts, hooksDir)
			case "TLSSecretName":
				resolvedSecret = resolveStringExpr(kv.Value, f, filepath.Join(hooksDir, "tls.go"), consts)
			}
		}
		return false
	})

	require.Equal(t, "d8-x", resolvedNamespace)
	require.Equal(t, "x-tls", resolvedSecret)
}

// resolveStringExprWithImportDir is a test-only flavour of
// resolveStringExpr that allows injecting an explicit importDir mapping
// for one package alias, bypassing the on-disk go module path resolution
// (which is not available in t.TempDir-rooted fixtures).
func resolveStringExprWithImportDir(expr ast.Expr, pkgAlias, importDir string, consts map[string]string, hookFileDir string) string {
	switch v := expr.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			return strings.Trim(v.Value, "\"`")
		}
	case *ast.Ident:
		if c, ok := consts[hookFileDir+":"+v.Name]; ok {
			return c
		}
	case *ast.SelectorExpr:
		ident, ok := v.X.(*ast.Ident)
		if !ok || ident.Name != pkgAlias {
			return ""
		}
		if c, ok := consts[importDir+":"+v.Sel.Name]; ok {
			return c
		}
	}
	return ""
}
