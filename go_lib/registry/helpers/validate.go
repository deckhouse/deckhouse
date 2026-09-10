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

	validation "github.com/go-ozzo/ozzo-validation/v4"
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
	// dnsNameRegexp matches the DNS name form of a host. A host here is either an
	// IP address or a DNS name, so no other character can legitimately appear.
	//
	// The optional trailing dot is the fully qualified form. "registry.example.com."
	// is a valid absolute name, resolvers and container runtimes accept it, and
	// an operator may well have written it -- so refusing it would reject a
	// working configuration rather than a dangerous one. It stays safe as a
	// directory name under /etc/containerd/registry.d, because a name that
	// begins with an alphanumeric can never be "." or "..".
	//
	// A leading or trailing hyphen is still refused, and not only because RFC
	// 1035 forbids it: the host becomes a directory name that shell commands in
	// the bashible steps pass as an argument, where a leading "-" reads as an
	// option.
	dnsNameRegexp = regexp.MustCompile(`^[0-9A-Za-z]([0-9A-Za-z._-]*[0-9A-Za-z])?\.?$`)

	// urlPathRegexp matches the repository path of a registry address. It is
	// deliberately narrower than RFC 3986: the module only ever derives it from
	// `imagesRepo`, whose own schema allows letters, digits, dots, dashes and
	// underscores.
	//
	// A segment of "." or ".." is excluded rather than merely resolved. The path
	// is joined into filesystem paths under /etc/containerd/registry.d and into
	// the `remoteurl` of the distribution configuration; a dot segment means
	// nothing to a registry and reaches outside the directory the module owns.
	urlPathRegexp = regexp.MustCompile(`^(/[0-9A-Za-z._-]+)*/?$`)

	// noProxyTokenRegexp matches one entry of a no_proxy list: a host, a domain
	// suffix, a CIDR block, or `*`.
	// The leading dot is the domain-suffix form: `.example.com` means "and
	// everything under it", which is what an operator writes and what the rule's
	// own description promises. It was refused before, so a legitimate no_proxy
	// was rejected.
	// The trailing slash is allowed for the same reason as the leading dot: the
	// schema for `proxy.noProxy` in candi/openapi/cluster_configuration.yaml
	// admits it (`^[a-z0-9\-\./]+$`), so an operator may have written
	// `example.com/` or `10.0.0.0/8/`. It is useless rather than dangerous --
	// the value becomes a NO_PROXY environment variable through `quote` -- and
	// refusing it would reject a configuration the platform accepts.
	noProxyTokenRegexp = regexp.MustCompile(`^\*$|^\.?[0-9A-Za-z*]([0-9A-Za-z*._:/-]*[0-9A-Za-z*])?/?$`)

	// accountNameRegexp matches a registry account name.
	accountNameRegexp = regexp.MustCompile(`^[0-9A-Za-z]([0-9A-Za-z._-]*[0-9A-Za-z])?$`)
)

const (
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
	NodeIPPlaceholder = "${discovered_node_ip}"

	// maxAccountNameLength bounds a registry account name well below the 1024
	// character limit YAML places on a simple key.
	maxAccountNameLength = 255
)

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

// stringRule adapts a check on a string into an ozzo rule.
//
// Every rule in this file shares the same preamble -- take the string, treat an
// absent value as valid, then check it -- so it lives here once. Empty is valid
// on purpose: presence is `validation.Required`'s job, and these rules are
// composed with it. A field that may legitimately be absent (the path component
// of an address, a proxy that is not configured, a CA on an HTTP upstream) would
// otherwise need a second, weaker rule for the absent case.
func stringRule(check func(string) error) validation.Rule {
	return validation.By(func(value any) error {
		raw, err := stringValue(value)
		if err != nil {
			return err
		}
		if raw == "" {
			return nil
		}
		return check(raw)
	})
}

// either accepts a value that satisfies any one of the rules.
//
// It exists for the fields that legitimately take more than one shape: a proxy
// endpoint is an `<ip>:<port>` pair or the bootstrap placeholder, and a mirror
// host is a registry host or that same placeholder. Spelling the alternatives
// out as separate rules keeps each one's own error message, and the combined
// failure names all of them -- which matters here, because "not an IP:port" on
// its own would send a reader looking for the wrong mistake.
func either(rules ...validation.Rule) validation.Rule {
	return validation.By(func(value any) error {
		messages := make([]string, 0, len(rules))
		for _, rule := range rules {
			err := rule.Validate(value)
			if err == nil {
				return nil
			}
			messages = append(messages, err.Error())
		}
		if len(messages) == 0 {
			return nil
		}
		return fmt.Errorf("must satisfy one of: %s", strings.Join(messages, "; "))
	})
}

// and accepts a value that satisfies every rule.
//
// A rule list is already a conjunction; this exists for the conjunctions that
// have to sit inside another combinator, such as one branch of an Either.
func and(rules ...validation.Rule) validation.Rule {
	return validation.By(func(value any) error {
		for _, rule := range rules {
			if err := rule.Validate(value); err != nil {
				return err
			}
		}
		return nil
	})
}

// EncodableString rejects values that cannot be represented in the YAML and TOML
// files the module generates. Such a value would be silently rewritten by the
// encoder, so that the configuration a node applies is not the one the module
// intended.
//
// Every control character is refused, the tab included. A tab does survive a
// quoted scalar, so this is stricter than the encoders require -- deliberately:
// nothing the module generates contains one, and a tab in a host or an account
// name is far more likely to be a mistake than an intention.
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
	return stringRule(validPort).Validate(value)
}

func validPort(raw string) error {
	for _, r := range raw {
		if r < '0' || r > '9' {
			// strconv.Atoi would accept a leading sign, and "+5001" reaches the
			// generated file verbatim, where nothing reads it as a port.
			return fmt.Errorf("port %q must be decimal digits", raw)
		}
	}
	number, err := strconv.Atoi(raw)
	if err != nil {
		return fmt.Errorf("port %q is not a number", raw)
	}
	if number < 1 || number > 65535 {
		return fmt.Errorf("port %d is out of range", number)
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
	return and(
		validation.By(EncodableString),
		stringRule(registryAccountName),
	).Validate(value)
}

func registryAccountName(raw string) error {
	if len(raw) > maxAccountNameLength {
		return fmt.Errorf("must be at most %d characters, got %d", maxAccountNameLength, len(raw))
	}
	if !accountNameRegexp.MatchString(raw) {
		return fmt.Errorf("must match %q", accountNameRegexp.String())
	}
	return nil
}

// HostWithOptionalPort validates `host` or `host:port`, where the host is an IP
// address or a DNS name and the port is optional.
//
// It is not specific to a registry: the same shape is a registry host, the host
// component of `imagesRepo`, and the authority of an HTTP proxy URL. The rule
// is written for the strictest of those sinks -- a registry host becomes a
// directory name under /etc/containerd/registry.d and a key in the generated
// hosts.toml, so it must carry neither a path separator nor anything a shell
// would interpret.
//
// The pair to it is IPWithPort, which requires a literal IP address and a port.
func HostWithOptionalPort(value any) error {
	return and(
		validation.By(EncodableString),
		stringRule(hostWithOptionalPort),
	).Validate(value)
}

func hostWithOptionalPort(raw string) error {
	if strings.ContainsRune(raw, '/') {
		return errors.New("must not contain a path separator")
	}

	host, port, err := splitHostPort(raw)
	if err != nil {
		return err
	}
	if port != "" {
		if err := validPort(port); err != nil {
			return err
		}
	}
	return validHostName(host)
}

// splitHostPort reads the three shapes a registry address takes: a bare host, a
// bare IP address, and `host:port`.
//
// The bare IP case is separate because net.SplitHostPort rejects an unbracketed
// IPv6 address -- "fd00::1" is a host, not a host and a port, and reading it as
// the latter is how a legitimate address gets refused. The port is returned
// empty when the value carries none.
func splitHostPort(raw string) (string, string, error) {
	// A bare IP address first, and before anything looks at the colons: "::" and
	// "fd00::1" are addresses, and the IPv6 wildcard even ends with a colon.
	if net.ParseIP(raw) != nil {
		return raw, "", nil
	}

	if splitHost, splitPort, splitErr := net.SplitHostPort(raw); splitErr == nil {
		// net.SplitHostPort reads "registry.example.com:" as a host with an
		// empty port. That value is malformed rather than portless, and the
		// difference is only visible here, so it is refused here.
		if splitPort == "" {
			return "", "", errors.New("has a trailing colon but no port")
		}
		return splitHost, splitPort, nil
	} else if strings.ContainsRune(raw, ':') {
		return "", "", fmt.Errorf("is not a valid host:port pair: %w", splitErr)
	}

	return raw, "", nil
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

// IPWithPort validates `<ip>:<port>` with a literal IP address. This is the only
// shape the module generates for a proxy endpoint, and the NGINX `server`
// directive that consumes it has no quoting of its own.
func IPWithPort(value any) error {
	return and(
		validation.By(EncodableString),
		stringRule(ipWithPort),
	).Validate(value)
}

func ipWithPort(raw string) error {
	// A port is required here, so net.SplitHostPort is the right reader: a bare
	// IPv6 address is not an endpoint the NGINX `server` directive can use
	// without one.
	host, port, err := net.SplitHostPort(raw)
	if err != nil {
		return fmt.Errorf("is not a valid host:port pair: %w", err)
	}
	if net.ParseIP(host) == nil {
		return fmt.Errorf("host %q is not an IP address", host)
	}
	return validPort(port)
}

// ProxyEndpoint validates one entry of `proxyEndpoints`.
//
// It is `<ip>:<port>`, or NodeIPPlaceholder in the host position during
// bootstrap. The value is interpolated into `server <value>;` in the node load
// balancer's NGINX configuration, which has no quoting of its own, and into an
// unquoted heredoc that runs as root.
func ProxyEndpoint(value any) error {
	return and(
		validation.By(EncodableString),
		either(
			validation.By(IPWithPort),
			validation.By(NodeIPPlaceholderWithPort),
		),
	).Validate(value)
}

// MirrorHost validates the host of a mirror in the bashible configuration.
//
// It is a registry host, or NodeIPPlaceholder in the host position during
// bootstrap. The value becomes a table key in
// /etc/containerd/registry.d/<host>/hosts.toml and part of the CA file name
// beside it, both written through an unquoted heredoc that runs as root.
func MirrorHost(value any) error {
	return and(
		validation.By(EncodableString),
		either(
			validation.By(HostWithOptionalPort),
			validation.By(NodeIPPlaceholderWithPort),
		),
	).Validate(value)
}

// NodeIPPlaceholderWithPort accepts NodeIPPlaceholder followed by a port, and
// nothing else.
//
// The placeholder is never generated bare: both producers build
// `<host>:<port>`, so requiring the port keeps the exception as narrow as what
// the module actually emits. It is an exported rule so that the alternatives in
// ProxyEndpoint and MirrorHost read as two named rules rather than as a special
// case buried in each.
func NodeIPPlaceholderWithPort(value any) error {
	return stringRule(func(raw string) error {
		host, port, err := net.SplitHostPort(raw)
		if err != nil || host != NodeIPPlaceholder {
			return fmt.Errorf("must be %s followed by a port", NodeIPPlaceholder)
		}
		return validPort(port)
	}).Validate(value)
}

// IPAddress validates a bare IP address.
func IPAddress(value any) error {
	return stringRule(func(raw string) error {
		if net.ParseIP(raw) == nil {
			return fmt.Errorf("%q is not an IP address", raw)
		}
		return nil
	}).Validate(value)
}

// URLScheme validates that a value is one of the two schemes the module supports.
// The scheme selects TLS handling in every consumer (skip_verify in hosts.toml,
// the upstream URL in the distribution configuration), so an unrecognised value
// must not reach them.
func URLScheme(value any) error {
	return stringRule(func(raw string) error {
		if raw != "http" && raw != "https" {
			return fmt.Errorf("must be http or https, got %q", raw)
		}
		return nil
	}).Validate(value)
}

// URLPath validates the repository path of a registry address.
//
// An absent path is valid, and deliberately so: `imagesRepo` may name a
// registry with no repository under it, and SplitAddressAndPath then yields an
// empty path. stringRule returns before the pattern is consulted, so the
// pattern never sees an empty string -- it would match one, since every group
// in it is optional, and the two agree rather than only appearing to.
func URLPath(value any) error {
	return and(
		validation.By(EncodableString),
		stringRule(urlPath),
	).Validate(value)
}

func urlPath(raw string) error {
	for _, segment := range strings.Split(raw, "/") {
		if segment == "." || segment == ".." {
			return fmt.Errorf("must not contain the path segment %q", segment)
		}
	}
	if !strings.HasPrefix(raw, "/") {
		return errors.New("must start with /")
	}
	if !urlPathRegexp.MatchString(raw) {
		return fmt.Errorf("must match %q", urlPathRegexp.String())
	}
	return nil
}

// RegistryAddress validates a `<host>[:<port>][/<path>]` registry address as a
// whole, the shape `imagesRepo` carries.
//
// It exists so that the boundary where the value enters the module agrees with
// the rules its sinks need, by construction rather than by two patterns kept in
// step by hand. The host becomes a directory name under
// /etc/containerd/registry.d and a table key in hosts.toml; the path is joined
// into the same filesystem paths and into the `remoteurl` of the distribution
// configuration.
func RegistryAddress(value any) error {
	return and(
		validation.By(EncodableString),
		stringRule(registryAddress),
	).Validate(value)
}

func registryAddress(raw string) error {
	host, path := SplitAddressAndPath(raw)

	if host == "" {
		return errors.New("has no host component")
	}
	if err := HostWithOptionalPort(host); err != nil {
		return fmt.Errorf("host %q is not valid: %w", host, err)
	}
	if err := URLPath(path); err != nil {
		return fmt.Errorf("path %q is not valid: %w", path, err)
	}
	return nil
}

// ProxyURL validates an HTTP proxy address: `http(s)://[user[:password]@]host[:port]`
// and nothing after that. A path, a query or a fragment are rejected.
//
// The shape is the one candi/openapi/cluster_configuration.yaml states for
// `proxy.httpProxy` and `proxy.httpsProxy`, which is where these values come
// from, and credentials are part of it -- its own examples are
// `https://user:password@proxy.company.my:8443` and the percent-encoded forms
// `DOMAIN%5Cuser` and `user%40domain.local`. A rule stricter than the schema
// rejects configurations the platform documents as valid, which is why this one
// follows the schema rather than the module's own preferences.
//
// Credentials are safe here because of where the value goes: the only sink in
// this module is a container environment variable in the static pod manifest,
// substituted through `quote`, so it is a YAML double-quoted scalar and no
// shell is involved. Were that to change -- were a proxy URL ever to reach an
// unquoted heredoc the way ProxyEndpoint and MirrorHost do -- this rule would
// have to constrain the userinfo characters as well, because the schema admits
// `$`, `(`, `)`, `;` and `&` there.
//
// A bare "/" is tolerated as the path, because it is the empty path: an operator
// who writes `http://proxy:8080/` has named the same proxy as
// `http://proxy:8080`, and every HTTP client treats the two alike.
func ProxyURL(value any) error {
	return and(
		validation.By(EncodableString),
		stringRule(proxyURL),
	).Validate(value)
}

func proxyURL(raw string) error {
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
	// The raw value is what gets rendered, not the parsed components, so the raw
	// value is what has to end at the authority. The component checks above do
	// not catch everything on their own: "http://host#" parses to a clean URL
	// with an empty fragment, yet still carries the "#" into the file.
	//
	// This is checked on the string rather than by comparing against
	// parsed.String(), because Go's re-encoding is not the schema's. A password
	// of "p@ss:word" is written `p%40ss:word`, which the schema allows and
	// parsed.String() rewrites to `p%40ss%3Aword` -- so a canonical-form
	// comparison would reject a documented value for a difference that exists
	// only inside net/url.
	prefix := parsed.Scheme + "://"
	if !strings.HasPrefix(strings.ToLower(raw), prefix) {
		// net/url lowercases the scheme and tolerates forms this does not, so
		// the offset of the authority is established rather than assumed.
		return fmt.Errorf("must begin with %q", prefix)
	}
	authority := strings.TrimSuffix(raw[len(prefix):], "/")
	if index := strings.IndexAny(authority, "/?#"); index >= 0 {
		return fmt.Errorf("must carry nothing after the host, got %q", authority[index:])
	}

	return HostWithOptionalPort(parsed.Host)
}

// NoProxyList validates a no_proxy value: a comma-separated list of hosts,
// domain suffixes or CIDR blocks.
func NoProxyList(value any) error {
	return and(
		validation.By(EncodableString),
		stringRule(noProxyList),
	).Validate(value)
}

func noProxyList(raw string) error {
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
