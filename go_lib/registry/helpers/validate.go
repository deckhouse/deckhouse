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

package helpers

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Rules for the values the registry module publishes to nodes.
//
// Every value validated here is later interpolated into a file that another
// component parses or executes: a static pod manifest read by kubelet, the
// NGINX configuration of the node load balancer, /etc/containerd/registry.d
// entries, or the distribution / docker_auth / mirrorer configurations. Values
// are escaped at the point of rendering, but escaping cannot help where the
// consumer has no quoting at all -- an NGINX `server` directive, or a directory
// name in a shell command. These rules therefore constrain each value to the
// shape the module actually produces, so that nothing else can reach a sink.
//
// Each rule is usable as an ozzo `validation.By` argument and treats an empty
// value as valid, leaving presence to `validation.Required`.

var (
	// dnsNameRegexp matches a DNS name component of a registry host. Registry
	// hosts are either an IP address or a DNS name, so no other character can
	// legitimately appear.
	dnsNameRegexp = regexp.MustCompile(`^[0-9A-Za-z]([0-9A-Za-z._-]*[0-9A-Za-z])?$`)

	// urlPathRegexp matches the repository path of a registry address. It is
	// deliberately narrower than RFC 3986: the module only ever derives it from
	// `imagesRepo`, whose own schema allows letters, digits, dots, dashes and
	// underscores.
	urlPathRegexp = regexp.MustCompile(`^(/[0-9A-Za-z._-]+)*/?$`)

	// noProxyTokenRegexp matches one entry of a no_proxy list: a host, a domain
	// suffix, a CIDR block, or `*`.
	noProxyTokenRegexp = regexp.MustCompile(`^\*|^[0-9A-Za-z*]([0-9A-Za-z*._:/-]*[0-9A-Za-z*])?$`)

	// accountNameRegexp matches a registry account name.
	accountNameRegexp = regexp.MustCompile(`^[0-9A-Za-z]([0-9A-Za-z._-]*[0-9A-Za-z])?$`)
)

// maxAccountNameLength bounds a registry account name well below the 1024
// character limit YAML places on a simple key.
const maxAccountNameLength = 255

// stringValue extracts the string an ozzo rule was given.
func stringValue(value any) (string, error) {
	switch typed := value.(type) {
	case string:
		return typed, nil
	case *string:
		if typed == nil {
			return "", nil
		}
		return *typed, nil
	default:
		return "", fmt.Errorf("must be a string, got %T", value)
	}
}

// EncodableString rejects values that cannot be represented in the YAML and TOML
// files the module generates. Such a value would be silently rewritten by the
// encoder, so that the configuration a node applies is not the one the module
// intended.
func EncodableString(value any) error {
	raw, err := stringValue(value)
	if err != nil {
		return err
	}
	if !utf8.ValidString(raw) {
		return errors.New("must be valid UTF-8")
	}
	for _, r := range raw {
		if r == '\n' || r == '\r' {
			return errors.New("must not contain line breaks")
		}
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("must not contain the control character %q", r)
		}
	}
	return nil
}

// Port validates a TCP port in decimal form.
func Port(value any) error {
	raw, err := stringValue(value)
	if err != nil {
		return err
	}
	if raw == "" {
		return nil
	}
	return validPort(raw)
}

func validPort(raw string) error {
	number, err := strconv.Atoi(raw)
	if err != nil {
		return fmt.Errorf("port %q is not a number", raw)
	}
	if number < 1 || number > 65535 {
		return fmt.Errorf("port %d is out of range", number)
	}
	return nil
}

func validHostName(host string) error {
	if host == "" {
		return errors.New("host is empty")
	}
	if net.ParseIP(host) != nil {
		return nil
	}
	if !dnsNameRegexp.MatchString(host) {
		return fmt.Errorf("host %q is neither an IP address nor a DNS name", host)
	}
	return nil
}

// RegistryAccountName validates a registry account name.
//
// The name becomes a mapping key in the docker_auth configuration and the
// `account` matcher of every ACL entry. YAML limits a simple key to 1024
// characters, so an unbounded name produces a configuration no parser accepts
// and the storage loses authentication entirely; the bound here stays well
// below that. The module generates `ro`, `rw`, `mirror-puller` and
// `mirror-pusher`.
func RegistryAccountName(value any) error {
	raw, err := stringValue(value)
	if err != nil {
		return err
	}
	if raw == "" {
		return nil
	}
	if len(raw) > maxAccountNameLength {
		return fmt.Errorf("must be at most %d characters, got %d", maxAccountNameLength, len(raw))
	}
	if !accountNameRegexp.MatchString(raw) {
		return fmt.Errorf("must match %q", accountNameRegexp.String())
	}
	return nil
}

// RegistryHost validates `host` or `host:port`, where the host is an IP address
// or a DNS name. The value becomes a directory name under
// /etc/containerd/registry.d and a key in the generated hosts.toml, so it must
// carry neither a path separator nor anything a shell would interpret.
func RegistryHost(value any) error {
	raw, err := stringValue(value)
	if err != nil {
		return err
	}
	if raw == "" {
		return nil
	}
	if err := EncodableString(raw); err != nil {
		return err
	}
	if strings.ContainsRune(raw, '/') {
		return errors.New("must not contain a path separator")
	}

	host := raw
	if splitHost, port, splitErr := net.SplitHostPort(raw); splitErr == nil {
		host = splitHost
		if err := validPort(port); err != nil {
			return err
		}
	} else if strings.ContainsRune(raw, ':') {
		return fmt.Errorf("is not a valid host:port pair: %w", splitErr)
	}

	return validHostName(host)
}

// IPPort validates `<ip>:<port>` with a literal IP address. This is the only
// shape the module generates for a proxy endpoint, and the NGINX `server`
// directive that consumes it has no quoting of its own.
func IPPort(value any) error {
	raw, err := stringValue(value)
	if err != nil {
		return err
	}
	if raw == "" {
		return nil
	}

	host, port, err := net.SplitHostPort(raw)
	if err != nil {
		return fmt.Errorf("is not a valid host:port pair: %w", err)
	}
	if net.ParseIP(host) == nil {
		return fmt.Errorf("host %q is not an IP address", host)
	}
	return validPort(port)
}

// NodeIPPlaceholder is the literal dhctl writes into the bashible registry
// configuration, in Proxy and Local mode, where a master node's own address
// belongs.
//
// During bootstrap there is no address to write. The registry that the node is
// about to pull from is the one it is about to start, and its address is only
// known on the node itself, so dhctl emits this placeholder and the bashible
// steps resolve it:
//
//	discovered_node_ip="$(bb-d8-node-ip)"
//
// The steps that consume it -- candi/bashible/common-steps/all/
// 001_configure_registry_proxy.sh.tpl and 030_configure_containerd_registry.sh.tpl
// -- therefore write their generated files through an *unquoted* heredoc, which
// is the only reason the substitution happens at all. That makes every value in
// those bodies subject to parameter and command substitution as root, and it is
// why ProxyEndpoint and MirrorHost exist: validation, not quoting, is what keeps
// a `$(...)` out of those files. This one literal is the sole exception, and it
// is compared exactly.
const NodeIPPlaceholder = "${discovered_node_ip}"

// ProxyEndpoint validates one entry of `proxyEndpoints`.
//
// It is `<ip>:<port>`, or NodeIPPlaceholder in the host position during
// bootstrap. The value is interpolated into `server <value>;` in the node load
// balancer's NGINX configuration, which has no quoting of its own, and into an
// unquoted heredoc that runs as root.
func ProxyEndpoint(value any) error {
	raw, err := stringValue(value)
	if err != nil {
		return err
	}
	if raw == "" {
		return nil
	}
	if err := EncodableString(raw); err != nil {
		return err
	}

	if port, ok := placeholderPort(raw); ok {
		return validPort(port)
	}
	return IPPort(raw)
}

// MirrorHost validates the host of a mirror in the bashible configuration.
//
// It is a registry host, or NodeIPPlaceholder in the host position during
// bootstrap. The value becomes a table key in
// /etc/containerd/registry.d/<host>/hosts.toml and part of the CA file name
// beside it, both written through an unquoted heredoc that runs as root.
func MirrorHost(value any) error {
	raw, err := stringValue(value)
	if err != nil {
		return err
	}
	if raw == "" {
		return nil
	}
	if err := EncodableString(raw); err != nil {
		return err
	}

	if port, ok := placeholderPort(raw); ok {
		return validPort(port)
	}
	return RegistryHost(raw)
}

// placeholderPort reports whether raw is NodeIPPlaceholder with a port, and
// returns that port. The placeholder is never generated bare: both producers
// build `<host>:<port>`, so requiring the port keeps the exception as narrow as
// what the module actually emits.
func placeholderPort(raw string) (string, bool) {
	host, port, err := net.SplitHostPort(raw)
	if err != nil || host != NodeIPPlaceholder {
		return "", false
	}
	return port, true
}

// IPAddress validates a bare IP address.
func IPAddress(value any) error {
	raw, err := stringValue(value)
	if err != nil {
		return err
	}
	if raw == "" {
		return nil
	}
	if net.ParseIP(raw) == nil {
		return fmt.Errorf("%q is not an IP address", raw)
	}
	return nil
}

// URLScheme validates that a value is one of the two schemes the module supports.
// The scheme selects TLS handling in every consumer (skip_verify in hosts.toml,
// the upstream URL in the distribution configuration), so an unrecognised value
// must not reach them.
func URLScheme(value any) error {
	raw, err := stringValue(value)
	if err != nil {
		return err
	}
	if raw == "" {
		return nil
	}
	if raw != "http" && raw != "https" {
		return fmt.Errorf("must be http or https, got %q", raw)
	}
	return nil
}

// URLPath validates the repository path of a registry address.
func URLPath(value any) error {
	raw, err := stringValue(value)
	if err != nil {
		return err
	}
	if raw == "" {
		return nil
	}
	if !strings.HasPrefix(raw, "/") {
		return errors.New("must start with /")
	}
	if !urlPathRegexp.MatchString(raw) {
		return fmt.Errorf("must match %q", urlPathRegexp.String())
	}
	return nil
}

// ProxyURL validates an HTTP proxy address: `http(s)://host[:port]` and nothing
// more. Credentials, a path, a query or a fragment are rejected, both because
// the module has no use for them and because they widen the set of characters
// that reach the generated configuration.
func ProxyURL(value any) error {
	raw, err := stringValue(value)
	if err != nil {
		return err
	}
	if raw == "" {
		return nil
	}
	if err := EncodableString(raw); err != nil {
		return err
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("is not a URL: %w", err)
	}
	// Both components are mandatory here, so they cannot be delegated to the
	// rules above: those treat an empty value as valid and leave presence to
	// validation.Required. A protocol-relative "//host" has no scheme and
	// "http://" has no host, and neither is a usable proxy address.
	if parsed.Scheme == "" {
		return errors.New("must be an absolute URL carrying an http or https scheme")
	}
	if err := URLScheme(parsed.Scheme); err != nil {
		return fmt.Errorf("scheme %w", err)
	}
	if parsed.Host == "" {
		return errors.New("must carry a host")
	}
	if parsed.User != nil {
		return errors.New("must not carry credentials")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return fmt.Errorf("must not carry a path, got %q", parsed.Path)
	}
	if parsed.RawQuery != "" || parsed.ForceQuery {
		return errors.New("must not carry a query")
	}
	if parsed.Fragment != "" {
		return errors.New("must not carry a fragment")
	}
	if parsed.Opaque != "" {
		return fmt.Errorf("must be a hierarchical URL, got %q", parsed.Opaque)
	}
	// The raw value is what gets rendered, not the parsed components, so it must
	// be the canonical form of the URL it parses to. Without this a value can
	// carry characters the component checks never see: "http://host#" parses to a
	// clean URL with an empty fragment, yet still contains the "#".
	if canonical := parsed.String(); canonical != raw {
		return fmt.Errorf("must be a canonical URL, got %q for %q", raw, canonical)
	}

	return RegistryHost(parsed.Host)
}

// NoProxyList validates a no_proxy value: a comma-separated list of hosts,
// domain suffixes or CIDR blocks.
func NoProxyList(value any) error {
	raw, err := stringValue(value)
	if err != nil {
		return err
	}
	if raw == "" {
		return nil
	}
	if err := EncodableString(raw); err != nil {
		return err
	}

	for _, token := range strings.Split(raw, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			return errors.New("must not contain an empty entry")
		}
		if !noProxyTokenRegexp.MatchString(token) {
			return fmt.Errorf("entry %q is not a host, domain suffix or CIDR block", token)
		}
	}
	return nil
}
