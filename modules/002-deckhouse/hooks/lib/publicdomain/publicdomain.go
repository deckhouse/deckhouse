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

// Package publicdomain describes the set of hostnames global.modules.publicDomainTemplate can
// render, which is what the deckhouse module reserves from tenant namespaces.
//
// templates/reserved-public-hosts.yaml derives the same set in Helm, because the admission policies
// need it as a regex a ConfigMap can carry. The derivation is the same by construction --
// sprig's regexQuoteMeta is regexp.QuoteMeta -- and
// template_tests/reserved_public_hosts_test.go compares the two so that they cannot drift.
package publicdomain

import (
	"fmt"
	"regexp"
	"strings"
)

// labelPattern is the RE2 fragment for what a service name may be, which is what the API server
// accepts as one DNS label. It is the %s of the template, so a hostname matches the pattern below
// exactly when some service name renders it.
const labelPattern = `[a-z0-9]([-a-z0-9]*[a-z0-9])?`

// HostPattern is the shape a hostname the reservation carries has to have. It is the literal
// modules/002-deckhouse/openapi/values.yaml validates every entry of
// deckhouse.internal.reservedPublicHosts.hosts against, kept here as well so that a hostname can be
// dropped before it reaches values: that key is fed from a ConfigMap an operator is invited to edit,
// and a value validation failure there would stop the module that renders Deckhouse from converging.
// publicdomain_test.go compares the two strings so that they cannot drift.
const HostPattern = `^(\*\.)?` + labelPattern + `(\.` + labelPattern + `)*$`

// The two reservations settings.reservedPublicHosts.mode selects between.
const (
	ModeTemplate = "Template"
	ModeList     = "List"
)

var hostRegexp = regexp.MustCompile(HostPattern)

// Namespace is the set of hostnames one publicDomainTemplate covers.
type Namespace struct {
	// Pattern matches every hostname the template can render.
	Pattern *regexp.Regexp
	// Wildcard is the wildcard form of the template, empty unless the template's %s is the whole
	// first label. Only then is the wildcard the template's namespace and nothing wider: with
	// "kube-%s.company.my" it would be "kube-*.company.my", which no API server accepts, while
	// "*.company.my" covers a domain the platform does not own.
	//
	// So a prefixed template leaves a gap an unprefixed one does not. "*.company.my" does match
	// "kube-anything.company.my", and a tenant holding it catches every prefixed platform hostname
	// nothing serves by exact match yet -- the window before the platform starts publishing one,
	// which is exactly what Template mode closes everywhere else. It cannot take a hostname the
	// platform already serves: the Gateway API gives the more specific hostname precedence, in so
	// many words ("foo.example.com" over "*.example.com", in the description of
	// Gateway.spec.listeners[].hostname in the CRD this repository ships at
	// ee/modules/110-istio/images/waypoint-controller/src/internal/waypointcontroller/crds_gateway_api/gateways.yaml).
	// The same is believed to hold for ingress-nginx through nginx's own server_name resolution
	// order, but that module is not in this repository and the claim is not verified here.
	//
	// Where admission-policy-engine is enabled and a SecurityPolicy sets blockWildcardDomains, the
	// gap is covered for Ingress by
	// modules/015-admission-policy-engine/charts/constraint-templates/templates/security/allowed-ingresses.yaml.
	// Nothing covers it for the Gateway API kinds.
	Wildcard string
}

// ParseNamespace derives the namespace of a publicDomainTemplate.
//
// The schema on global.modules.publicDomainTemplate (global-hooks/openapi/config-values.yaml) admits
// exactly one %s and only inside the first label, so splitting on %s yields a prefix and a suffix.
// An error means the value never went through that schema, and the caller must not treat the result
// as a reservation.
func ParseNamespace(domainTemplate string) (Namespace, error) {
	parts := strings.Split(strings.ToLower(domainTemplate), "%s")
	if len(parts) != 2 {
		return Namespace{}, fmt.Errorf("publicDomainTemplate %q does not hold exactly one %%s", domainTemplate)
	}
	prefix, suffix := parts[0], parts[1]

	pattern, err := regexp.Compile(fmt.Sprintf("^%s%s%s$",
		regexp.QuoteMeta(prefix), labelPattern, regexp.QuoteMeta(suffix)))
	if err != nil {
		// Unreachable: everything outside the fixed skeleton went through QuoteMeta.
		return Namespace{}, fmt.Errorf("derive pattern from publicDomainTemplate %q: %w", domainTemplate, err)
	}

	namespace := Namespace{Pattern: pattern}
	if prefix == "" && strings.HasPrefix(suffix, ".") {
		namespace.Wildcard = "*" + suffix
	}
	return namespace, nil
}

// EffectiveMode is the reservation actually in force, which is not always the one the ModuleConfig
// asked for: a publicDomainTemplate that does not split into a prefix and a suffix never went
// through the global schema, and a reservation must not be derived from parts that are not there, so
// it falls back to List.
//
// templates/reserved-public-hosts.yaml decides the same thing in Helm and publishes the answer in
// the mode key of the ConfigMap. It has to be decided the same way on both sides: the record the
// snapshot hook writes is applied only under Template, so a hook that recorded while the template
// reserved by List would leave a record taken under one reservation feeding another.
// template_tests/reserved_public_hosts_test.go compares the two so that they cannot drift.
func EffectiveMode(configured, domainTemplate string) string {
	if configured == ModeList {
		return ModeList
	}
	if _, err := ParseNamespace(domainTemplate); err != nil {
		return ModeList
	}
	return ModeTemplate
}

// Covers reports whether the hostname is one the reservation claims. The hostname is expected to be
// normalized already.
func (n Namespace) Covers(host string) bool {
	if n.Pattern == nil {
		return false
	}
	return host == n.Wildcard && n.Wildcard != "" || n.Pattern.MatchString(host)
}

// PatternCovers reports whether the hostname is claimed by the derived pattern, which is the part of
// the reservation an allowlist can lift. The wildcard form is claimed by exact match instead and is
// never given back, since holding it shadows every hostname the template can render, including the
// ones the platform learns to publish later. The hostname is expected to be normalized already.
func (n Namespace) PatternCovers(host string) bool {
	if n.Pattern == nil {
		return false
	}
	return n.Pattern.MatchString(host)
}

// NormalizeHost spells a hostname the way the reservation compares it, matching the lowerAscii and
// the root-dot trim the admission policies apply to what a request claims.
func NormalizeHost(host string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
}

// IsHost reports whether the string is a hostname the reservation can carry, which is the shape
// openapi/values.yaml admits. Expects a normalized hostname: a spelling NormalizeHost would have
// fixed is reported as garbage rather than repaired.
func IsHost(host string) bool {
	return hostRegexp.MatchString(host)
}
