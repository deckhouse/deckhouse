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

package upstreams

import (
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// The per-field limits of a RegistryUpstream spec. The CRD
// (modules/038-registry/crds/registryupstream.yaml) declares the same, and so does every
// copy of the spec on a node (NodeConfig spec.registry.upstreams, in nodelet's schema and
// loader): checking them here as well keeps an object that predates a limit, or reaches
// the arbitration another way, from being accepted and then refused on every node.
const (
	MaxMatchLength    = 255
	MaxHostLength     = 255
	MaxPathLength     = 1024
	MaxCALength       = 65536
	MaxUsernameLength = 1024
	MaxPasswordLength = 4096
	MaxAuthLength     = 4096
	MaxMirrors        = 8
)

var (
	// matchPattern is spec.match's pattern in the CRD: a DNS name or an IPv4 address,
	// optionally with a port.
	matchPattern = regexp.MustCompile(
		`^[a-zA-Z0-9]([-a-zA-Z0-9]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([-a-zA-Z0-9]*[a-zA-Z0-9])?)*(:[0-9]{1,5})?$`)

	// hostPattern is a registry host as nodelet takes it (internal/config/registry.go,
	// registryHostPattern): the match pattern, or a bracketed IPv6 address.
	hostPattern = regexp.MustCompile(
		`^([a-zA-Z0-9]([-a-zA-Z0-9]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([-a-zA-Z0-9]*[a-zA-Z0-9])?)*|\[[0-9A-Fa-f:.]+\])(:[0-9]{1,5})?$`)

	// repositoryPathPattern is a repository name of the distribution reference grammar,
	// as nodelet checks a path without its slashes (internal/config/registry.go).
	repositoryPathPattern = regexp.MustCompile(
		`^[a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*(/[a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*)*$`)

	// base64Pattern is a pre-encoded pair as bashible-apiserver renders it into a shell
	// script (pkg/template/registry/upstreams.go, base64Regexp): no line breaks, which
	// base64.StdEncoding would skip.
	base64Pattern = regexp.MustCompile(`^[A-Za-z0-9+/]*={0,2}$`)
)

// ValidateMatch checks spec.match the way the CRD does.
func ValidateMatch(match string) error {
	if n := utf8.RuneCountInString(match); n > MaxMatchLength {
		return fmt.Errorf("%d characters, longer than %d", n, MaxMatchLength)
	}
	if !matchPattern.MatchString(match) {
		return fmt.Errorf("%q is not a registry host, optionally with a port", match)
	}
	return validateHostParts(match)
}

// validateHostParts checks what the patterns cannot: the port is 1-65535, and brackets
// hold an IPv6 address followed by a port. The rule bashible-apiserver renders a host for
// containerd v1 by (go_lib/registry helpers.HostWithOptionalPort, not imported here).
func validateHostParts(host string) error {
	name, port := host, ""
	if strings.HasPrefix(host, "[") {
		end := strings.LastIndex(host, "]")
		name, port = host[1:end], strings.TrimPrefix(host[end+1:], ":")
		if ip := net.ParseIP(name); ip == nil || !strings.Contains(name, ":") {
			return fmt.Errorf("brackets enclose %q, which is not an IPv6 address", name)
		}
		if port == "" {
			return fmt.Errorf("IPv6 address %q has no port", host)
		}
	} else if i := strings.LastIndex(host, ":"); i >= 0 {
		name, port = host[:i], host[i+1:]
	}
	if port == "" {
		return nil
	}
	if number, err := strconv.Atoi(port); err != nil || number < 1 || number > 65535 {
		return fmt.Errorf("port %s of %q is out of range 1-65535", port, name)
	}
	return nil
}

// ValidateUpstream checks spec.upstream: each address the way a node writes it into a
// containerd hosts.toml, and the addresses together.
//
// Repeated addresses are refused because a node keys the [host] tables of one route by
// scheme and host: the second table would overwrite the first, or make the file
// unparsable.
func ValidateUpstream(upstream *registryv1alpha1.Upstream) error {
	if n := len(upstream.Mirrors); n > MaxMirrors {
		return fmt.Errorf("spec.upstream.mirrors: %d mirrors, more than %d", n, MaxMirrors)
	}

	endpoints := upstream.Endpoints()
	seen := make(map[string]string, len(endpoints))
	for i := range endpoints {
		at := "spec.upstream"
		if i > 0 {
			at = fmt.Sprintf("spec.upstream.mirrors[%d]", i-1)
		}
		if err := ValidateEndpoint(&endpoints[i]); err != nil {
			return fmt.Errorf("%s: %w", at, err)
		}

		address := endpointScheme(&endpoints[i]) + "://" + NormalizeHost(endpoints[i].Host)
		if first, repeated := seen[address]; repeated {
			return fmt.Errorf("%s: %s is named by %s already", at, address, first)
		}
		seen[address] = at
	}
	return nil
}

// ValidateEndpoint checks one address of an upstream.
func ValidateEndpoint(endpoint *registryv1alpha1.Endpoint) error {
	switch endpoint.Scheme {
	case "", registryv1alpha1.SchemeHTTP, registryv1alpha1.SchemeHTTPS:
	default:
		return fmt.Errorf("scheme %q is neither HTTP nor HTTPS", endpoint.Scheme)
	}

	if err := maxLength("host", endpoint.Host, MaxHostLength); err != nil {
		return err
	}
	if !hostPattern.MatchString(endpoint.Host) {
		return fmt.Errorf("host %q is not a registry host, optionally with a port", endpoint.Host)
	}
	if err := validateHostParts(endpoint.Host); err != nil {
		return fmt.Errorf("host: %w", err)
	}

	if err := maxLength("path", endpoint.Path, MaxPathLength); err != nil {
		return err
	}
	if path := strings.Trim(endpoint.Path, "/"); path != "" && !repositoryPathPattern.MatchString(path) {
		return fmt.Errorf("path %q is not a repository path", endpoint.Path)
	}

	if err := maxLength("ca", endpoint.CA, MaxCALength); err != nil {
		return err
	}
	if strings.TrimSpace(endpoint.CA) != "" {
		if err := ValidateCertificates(endpoint.CA); err != nil {
			return fmt.Errorf("ca: %w", err)
		}
	}

	return validateAuth(endpoint.Auth)
}

// ValidateCertificates accepts a PEM bundle of certificates and nothing else: at least
// one block, every block a CERTIFICATE that parses, nothing but whitespace around them.
// bashible-apiserver renders a CA for containerd v1 by the same rule
// (pkg/template/registry/upstreams.go, canonicalCertificates), and containerd fails
// every pull of a registry whose CA file it cannot load.
func ValidateCertificates(bundle string) error {
	rest := []byte(bundle)
	found := 0
	for {
		block, remaining := pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			return fmt.Errorf("a PEM block of type %q, want CERTIFICATE", block.Type)
		}
		if _, err := x509.ParseCertificate(block.Bytes); err != nil {
			return fmt.Errorf("parse certificate: %w", err)
		}
		found++
		rest = remaining
	}
	if len(bytes.TrimSpace(rest)) != 0 || found == 0 {
		return errors.New("not a PEM bundle of certificates")
	}
	return nil
}

// validateAuth checks the credentials of an address. The pre-encoded form must decode to
// the "username:password" pair it stands for: containerd sends it as it is, and the
// registry agent splits it (Auth.BasicCredentials), so a value that is not such a pair
// authenticates on one path and goes out anonymous on the other. With both forms given,
// they must name the same pair, or which one a node uses would decide who it pulls as.
func validateAuth(auth *registryv1alpha1.Auth) error {
	if auth == nil {
		return nil
	}
	if !auth.SecretRef.IsEmpty() {
		return errors.New("auth.secretRef is not served for a RegistryUpstream, and a node cannot resolve it")
	}
	if err := maxLength("auth.username", auth.Username, MaxUsernameLength); err != nil {
		return err
	}
	if err := maxLength("auth.password", auth.Password, MaxPasswordLength); err != nil {
		return err
	}
	if err := maxLength("auth.auth", auth.Auth, MaxAuthLength); err != nil {
		return err
	}
	if auth.Auth == "" {
		return nil
	}

	decoded, err := base64.StdEncoding.DecodeString(auth.Auth)
	if err != nil || !base64Pattern.MatchString(auth.Auth) {
		return errors.New("auth.auth is not base64 on one line")
	}
	if !strings.Contains(string(decoded), ":") {
		return errors.New(`auth.auth does not decode to "username:password"`)
	}
	if (auth.Username != "" || auth.Password != "") && string(decoded) != auth.Username+":"+auth.Password {
		return errors.New("auth.auth and auth.username with auth.password name different credentials")
	}
	return nil
}

func maxLength(field, value string, limit int) error {
	if n := utf8.RuneCountInString(value); n > limit {
		return fmt.Errorf("%s is %d characters, longer than %d", field, n, limit)
	}
	return nil
}

// endpointScheme is the URL scheme an address is reached with, https unless it says http.
func endpointScheme(endpoint *registryv1alpha1.Endpoint) string {
	if endpoint.Scheme == registryv1alpha1.SchemeHTTP {
		return "http"
	}
	return "https"
}
