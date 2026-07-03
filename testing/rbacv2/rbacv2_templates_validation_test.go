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

// Package rbacv2 validates the RBACv2 role/capability framework invariants across
// every module's templates/rbacv2 directory. The framework relies on a strict
// label contract (rbac.deckhouse.io/kind, rbac.deckhouse.io/scope, the
// aggregate-to-<lineage>-as aggregation labels) which is spread over ~50 modules
// and cannot be enforced by the API server, so a divergent module would silently
// break role aggregation. This test locks the contract in CI.
package rbacv2

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/yaml"
)

const labelPrefix = "rbac.deckhouse.io/"

var (
	validLevels = set("viewer", "user", "manager", "admin", "superadmin")
	subsystems  = set("deckhouse", "infrastructure", "kubernetes", "networking", "observability", "security", "storage")
	validScopes = set("system", "subsystem", "namespace", "project")

	// Lineages a capability or role may aggregate into: the system lineage, the
	// namespace/project lineages, and one lineage per subsystem.
	validLineages = union(set("system", "namespace", "project"), subsystems)

	aggregateLabelRe = regexp.MustCompile(`^rbac\.deckhouse\.io/aggregate-to-([a-z0-9-]+)-as$`)

	// labelValueRe is the Kubernetes label-value grammar; the per-capability
	// rbac.deckhouse.io/capability marker must satisfy it (and be <= 63 chars).
	labelValueRe = regexp.MustCompile(`^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?$`)

	roleNameRe = map[string]*regexp.Regexp{
		"system":    regexp.MustCompile(`^d8:system:([a-z]+)$`),
		"subsystem": regexp.MustCompile(`^d8:subsystem:([a-z0-9-]+):([a-z]+)$`),
		"namespace": regexp.MustCompile(`^d8:namespace:([a-z]+)$`),
		"project":   regexp.MustCompile(`^d8:project:([a-z]+)$`),
	}

	capabilityNamePrefix = map[string]string{
		"system":    "d8:system-capability:",
		"subsystem": "d8:subsystem-capability:",
		"namespace": "d8:namespace-capability:",
		"project":   "d8:project-capability:",
	}

	// helmLabelPairRe extracts `"key" "value"` pairs from the
	// helm_lib_module_labels (dict ...) invocation used by a few modules.
	helmLabelPairRe = regexp.MustCompile(`"(rbac\.deckhouse\.io/[^"]+)"\s+"([^"]+)"`)
	// plainLabelRe extracts ordinary `key: value` label lines for templated
	// files that keep their labels as plain YAML.
	plainLabelRe = regexp.MustCompile(`(?m)^\s+(rbac\.deckhouse\.io/[a-z0-9-]+):\s*"?([^"\s]+)"?\s*$`)
	helmNameRe   = regexp.MustCompile(`(?m)^\s{2}name:\s+(\S+)\s*$`)
)

// clusterRoleFile is a normalized view of a single rbacv2 template file. Files
// templated with helm_lib_module_labels cannot be YAML-parsed, so for them the
// labels and the name are extracted with regexes and the aggregationRule is not
// inspected (none of them define one).
type clusterRoleFile struct {
	path        string
	name        string
	labels      map[string]string
	raw         string
	hasRules    bool
	helmLabeled bool
	aggregation *rbacv1.AggregationRule
}

func TestRBACv2TemplatesValidation(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	files := collectRBACv2Files(t, root)
	if len(files) < 100 {
		t.Fatalf("found only %d rbacv2 template files under %s, expected at least 100; the discovery logic is probably broken", len(files), root)
	}

	var errs []string
	// capValues tracks the rbac.deckhouse.io/capability marker across every
	// capability so the constructor in console can aggregate a single capability
	// by a globally unique label.
	capValues := map[string][]string{}
	for _, path := range files {
		obj, err := parseFile(path)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", rel(root, path), err))
			continue
		}
		if obj.labels[labelPrefix+"kind"] == "capability" {
			if v := obj.labels[labelPrefix+"capability"]; v != "" {
				capValues[v] = append(capValues[v], rel(root, path))
			}
		}
		for _, problem := range validate(obj) {
			errs = append(errs, fmt.Sprintf("%s: %s", rel(root, path), problem))
		}
	}
	for value, paths := range capValues {
		if len(paths) > 1 {
			errs = append(errs, fmt.Sprintf("rbac.deckhouse.io/capability value %q is not unique, used by: %s", value, strings.Join(paths, ", ")))
		}
	}

	if len(errs) > 0 {
		t.Errorf("rbacv2 template contract violations (%d):\n  %s", len(errs), strings.Join(errs, "\n  "))
	}
}

func collectRBACv2Files(t *testing.T, root string) []string {
	var files []string
	for _, dir := range []string{"modules", "ee"} {
		base := filepath.Join(root, dir)
		// Some editions/test images (e.g. OSS) ship without the ee/ tree; skip
		// top-level directories that are not present in this build.
		if _, statErr := os.Stat(base); os.IsNotExist(statErr) {
			continue
		}
		err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if strings.HasSuffix(path, ".yaml") && strings.Contains(path, string(filepath.Separator)+"templates"+string(filepath.Separator)+"rbacv2"+string(filepath.Separator)) {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return files
}

func parseFile(path string) (*clusterRoleFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	raw := string(data)

	if strings.Contains(raw, "{{") {
		return parseHelmLabeledFile(path, raw)
	}

	var role rbacv1.ClusterRole
	if err := yaml.UnmarshalStrict([]byte(strings.TrimPrefix(raw, "---\n")), &role); err != nil {
		return nil, fmt.Errorf("failed to parse as ClusterRole: %w", err)
	}
	return &clusterRoleFile{
		path:        path,
		name:        role.Name,
		labels:      role.Labels,
		raw:         raw,
		hasRules:    len(role.Rules) > 0,
		aggregation: role.AggregationRule,
	}, nil
}

func parseHelmLabeledFile(path, raw string) (*clusterRoleFile, error) {
	nameMatch := helmNameRe.FindStringSubmatch(raw)
	if nameMatch == nil {
		return nil, fmt.Errorf("cannot extract metadata.name from helm-templated file")
	}
	labels := map[string]string{}
	for _, m := range helmLabelPairRe.FindAllStringSubmatch(raw, -1) {
		labels[m[1]] = m[2]
	}
	for _, m := range plainLabelRe.FindAllStringSubmatch(raw, -1) {
		labels[m[1]] = m[2]
	}
	if len(labels) == 0 {
		return nil, fmt.Errorf("cannot extract rbac.deckhouse.io labels from helm-templated file")
	}
	return &clusterRoleFile{
		path:        path,
		name:        nameMatch[1],
		labels:      labels,
		raw:         raw,
		hasRules:    regexp.MustCompile(`(?m)^rules:`).MatchString(raw) && !strings.Contains(raw, "rules: []"),
		helmLabeled: true,
	}, nil
}

func validate(obj *clusterRoleFile) []string {
	var errs []string
	fail := func(format string, args ...any) { errs = append(errs, fmt.Sprintf(format, args...)) }

	if !strings.HasPrefix(obj.name, "d8:") {
		fail("name %q must start with the d8: prefix", obj.name)
	}

	if !strings.Contains(obj.raw, "en.meta.deckhouse.io/title") || !strings.Contains(obj.raw, "ru.meta.deckhouse.io/title") {
		fail("missing en/ru.meta.deckhouse.io/title i18n annotations")
	}
	if !strings.Contains(obj.raw, "en.meta.deckhouse.io/description") || !strings.Contains(obj.raw, "ru.meta.deckhouse.io/description") {
		fail("missing en/ru.meta.deckhouse.io/description i18n annotations")
	}

	// d8:dict is a standalone helper role bound automatically by the
	// handle_dict_bindings hook; it intentionally lives outside the
	// role/capability framework and carries no kind/scope labels.
	if obj.name == "d8:dict" {
		return errs
	}

	kind := obj.labels[labelPrefix+"kind"]
	scope := obj.labels[labelPrefix+"scope"]

	if kind != "role" && kind != "capability" {
		fail("label %skind must be \"role\" or \"capability\", got %q", labelPrefix, kind)
		return errs
	}
	if !validScopes[scope] {
		fail("label %sscope must be one of system/subsystem/namespace/project, got %q", labelPrefix, scope)
		return errs
	}

	switch kind {
	case "role":
		errs = append(errs, validateRole(obj, scope)...)
	case "capability":
		errs = append(errs, validateCapability(obj, scope)...)
	}

	// Aggregation labels: the lineage must exist and the target level must be valid.
	for key, value := range obj.labels {
		m := aggregateLabelRe.FindStringSubmatch(key)
		if m == nil {
			continue
		}
		if !validLineages[m[1]] {
			fail("aggregation label %q targets unknown lineage %q", key, m[1])
		}
		if !validLevels[value] {
			fail("aggregation label %q has invalid level %q", key, value)
		}
	}

	// The delegatable marker is consumed by the multitenancy-manager grants engine
	// and must only appear on bindable namespace/project roles.
	if _, ok := obj.labels[labelPrefix+"delegatable"]; ok {
		if kind != "role" || (scope != "namespace" && scope != "project") {
			fail("label %sdelegatable is only allowed on namespace/project roles", labelPrefix)
		}
	}

	return errs
}

func validateRole(obj *clusterRoleFile, scope string) []string {
	var errs []string
	fail := func(format string, args ...any) { errs = append(errs, fmt.Sprintf(format, args...)) }

	re := roleNameRe[scope]
	m := re.FindStringSubmatch(obj.name)
	if m == nil {
		fail("role name %q does not match the %s-scope pattern %s", obj.name, scope, re)
		return errs
	}
	level := m[len(m)-1]
	if !validLevels[level] {
		fail("role name %q has invalid level %q", obj.name, level)
	}
	if scope == "subsystem" {
		if !subsystems[m[1]] {
			fail("role name %q references unknown subsystem %q", obj.name, m[1])
		}
		if got := obj.labels[labelPrefix+"subsystem"]; got != m[1] {
			fail("label %ssubsystem %q does not match the subsystem %q from the role name", labelPrefix, got, m[1])
		}
	}

	// system/subsystem roles fan out namespaced RoleBindings via the
	// handle_manage_bindings hook, which requires the use-role mapping.
	if scope == "system" || scope == "subsystem" {
		if useRole := obj.labels[labelPrefix+"use-role"]; !validLevels[useRole] {
			fail("label %suse-role must carry a valid level, got %q", labelPrefix, obj.labels[labelPrefix+"use-role"])
		}
	}

	if obj.hasRules {
		fail("role %q must not define its own rules; move them into a capability", obj.name)
	}
	if obj.aggregation == nil || len(obj.aggregation.ClusterRoleSelectors) == 0 {
		fail("role %q must define aggregationRule.clusterRoleSelectors", obj.name)
		return errs
	}
	for _, selector := range obj.aggregation.ClusterRoleSelectors {
		for key, value := range selector.MatchLabels {
			m := aggregateLabelRe.FindStringSubmatch(key)
			if m == nil {
				fail("role %q aggregation selector uses non-aggregation label %q", obj.name, key)
				continue
			}
			if !validLineages[m[1]] {
				fail("role %q aggregation selector targets unknown lineage %q", obj.name, m[1])
			}
			if !validLevels[value] {
				fail("role %q aggregation selector has invalid level %q", obj.name, value)
			}
		}
	}
	return errs
}

func validateCapability(obj *clusterRoleFile, scope string) []string {
	var errs []string
	fail := func(format string, args ...any) { errs = append(errs, fmt.Sprintf(format, args...)) }

	if prefix := capabilityNamePrefix[scope]; !strings.HasPrefix(obj.name, prefix) {
		fail("capability name %q must start with %q for scope %q", obj.name, prefix, scope)
	}
	if !obj.hasRules {
		fail("capability %q must define rules", obj.name)
	}
	if obj.aggregation != nil {
		fail("capability %q must not define aggregationRule", obj.name)
	}
	if len(filterAggregationLabels(obj.labels)) == 0 {
		fail("capability %q does not aggregate into any role (no aggregate-to-*-as labels)", obj.name)
	}
	// Each capability must carry a unique rbac.deckhouse.io/capability marker so
	// a custom role can selectively aggregate exactly this capability (k8s
	// aggregation matches on labels, and the other labels are shared by tier).
	marker := obj.labels[labelPrefix+"capability"]
	switch {
	case marker == "":
		fail("capability %q must carry the %scapability label", obj.name, labelPrefix)
	case len(marker) > 63 || !labelValueRe.MatchString(marker):
		fail("capability %q has invalid %scapability label value %q", obj.name, labelPrefix, marker)
	}
	return errs
}

func filterAggregationLabels(labels map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range labels {
		if aggregateLabelRe.MatchString(key) {
			out[key] = value
		}
	}
	return out
}

func rel(root, path string) string {
	r, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return r
}

func set(items ...string) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, item := range items {
		out[item] = true
	}
	return out
}

func union(a, b map[string]bool) map[string]bool {
	out := make(map[string]bool, len(a)+len(b))
	for k := range a {
		out[k] = true
	}
	for k := range b {
		out[k] = true
	}
	return out
}
