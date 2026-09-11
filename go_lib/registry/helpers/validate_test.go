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

// Unit tests for the validation rules.
//
// The fuzz harnesses next to these tests state properties -- "anything accepted
// is safe for the sink it reaches" -- and find the inputs that break them. What
// they cannot state is the intent: which shapes each rule is meant to accept.
// That is what this file pins, so a rule that is tightened or widened shows the
// change here as a named case rather than as a fuzz failure with a random input.

package helpers

import (
	"strings"
	"testing"

	validation "github.com/go-ozzo/ozzo-validation/v4"
)

// ruleCase is one input and whether the rule under test must accept it.
type ruleCase struct {
	in     any
	accept bool
	why    string
}

func runRule(t *testing.T, name string, rule func(any) error, cases []ruleCase) {
	t.Helper()

	t.Run(name, func(t *testing.T) {
		for _, c := range cases {
			err := rule(c.in)
			if c.accept && err != nil {
				t.Errorf("%s(%#v) rejected a value it must accept: %v (%s)", name, c.in, err, c.why)
			}
			if !c.accept && err == nil {
				t.Errorf("%s(%#v) accepted a value it must reject (%s)", name, c.in, c.why)
			}
		}
	})
}

// TestEmptyIsValidEverywhere pins the one convention every rule shares.
//
// Empty is accepted by design: presence belongs to validation.Required, which
// the models compose these rules with. Several of the fields are legitimately
// absent -- the path component of an address, a proxy that is not configured, a
// CA on an HTTP upstream -- and a rule that rejected empty would force each of
// those to carry a second, weaker rule for the absent case.
func TestEmptyIsValidEverywhere(t *testing.T) {
	rules := map[string]func(any) error{
		"EncodableString":           EncodableString,
		"Port":                      Port,
		"RegistryAccountName":       RegistryAccountName,
		"HostWithOptionalPort":      HostWithOptionalPort,
		"IPWithPort":                IPWithPort,
		"ProxyEndpoint":             ProxyEndpoint,
		"MirrorHost":                MirrorHost,
		"NodeIPPlaceholderWithPort": NodeIPPlaceholderWithPort,
		"IPAddress":                 IPAddress,
		"URLScheme":                 URLScheme,
		"URLPath":                   URLPath,
		"RegistryAddress":           RegistryAddress,
		"ProxyURL":                  ProxyURL,
		"NoProxyList":               NoProxyList,
	}

	for name, rule := range rules {
		if err := rule(""); err != nil {
			t.Errorf("%s(\"\") must accept an absent value, got %v", name, err)
		}

		var absent *string
		if err := rule(absent); err != nil {
			t.Errorf("%s(nil *string) must accept an absent value, got %v", name, err)
		}

		if err := rule(42); err == nil {
			t.Errorf("%s(42) must report that it was not given a string", name)
		}
	}
}

// TestURLPathEmptyAgreesWithItsPattern is the second half of the answer.
//
// checkString returns before the pattern is consulted, so URLPath never asks
// the pattern about an empty string. The pattern would match one anyway -- every
// group in it is optional -- so the guard and the pattern agree rather than only
// appearing to. A future change that removed the guard would therefore not
// change the verdict, which is why the pattern is asserted directly here.
func TestURLPathEmptyAgreesWithItsPattern(t *testing.T) {
	if err := URLPath(""); err != nil {
		t.Fatalf("URLPath(\"\") must accept: %v", err)
	}
	if !urlPathRegexp.MatchString("") {
		t.Fatal("urlPathRegexp must also match the empty string, so that removing " +
			"the empty-value guard in checkString would not silently change URLPath")
	}
}

func TestURLPath(t *testing.T) {
	runRule(t, "URLPath", URLPath, []ruleCase{
		{in: "/system/deckhouse", accept: true, why: "the shape imagesRepo yields"},
		{in: "/deckhouse", accept: true, why: "a single segment"},
		{in: "/a/b/c/d", accept: true, why: "several segments"},
		{in: "/system/deckhouse/", accept: true, why: "a trailing slash is tolerated"},
		{in: "/", accept: true, why: "a root path is an empty path with a separator"},
		{in: "/a.b", accept: true, why: "dots inside a segment are part of a repository name"},
		{in: "/a-b_c", accept: true, why: "dashes and underscores are too"},
		{in: "system/deckhouse", why: "a path must be absolute; a relative one would join wrongly"},
		{in: "//", why: "an empty segment"},
		{in: "/a//b", why: "an empty segment between two others"},
		{in: "/.", why: "a dot segment means nothing to a registry"},
		{in: "/..", why: "a dot-dot segment reaches outside the directory the module owns"},
		{in: "/a/../b", why: "the same, in the middle"},
		{in: "/a/./b", why: "and the single-dot form"},
		{in: "/a b", why: "a space would end a token in the files this is written into"},
		{in: "/a;b", why: "a separator of the sinks downstream"},
		{in: "/a\nb", why: "a line break would add a line to a generated file"},
		{in: "/$(id)", why: "a shell substitution"},
		{in: "/a\x00b", why: "a control character"},
		{in: "/" + strings.Repeat("a", 300), accept: true, why: "length is not what this rule bounds"},
	})
}

// TestHostWithOptionalPortIPv6 pins the case net.SplitHostPort cannot read.
//
// A bare IPv6 address contains colons, so SplitHostPort reports an error for it
// and a rule that took that error as "malformed host:port" would refuse a
// perfectly good host. splitHostPort tries net.ParseIP first for exactly this.
func TestHostWithOptionalPortIPv6(t *testing.T) {
	for _, host := range []string{"::1", "fd00::1", "2001:db8::8a2e:370:7334", "::"} {
		if err := HostWithOptionalPort(host); err != nil {
			t.Errorf("HostWithOptionalPort(%q) must accept a bare IPv6 address: %v", host, err)
		}
		if err := MirrorHost(host); err != nil {
			t.Errorf("MirrorHost(%q) must accept a bare IPv6 address: %v", host, err)
		}
	}

	// With a port it is the bracketed form, which SplitHostPort does read.
	if err := HostWithOptionalPort("[fd00::1]:5001"); err != nil {
		t.Errorf("HostWithOptionalPort(\"[fd00::1]:5001\") must accept: %v", err)
	}

	// An unbracketed value that looks like an address with a port is read as the
	// address it is: "fd00::1:5001" is a valid IPv6 address, and there is no way
	// to tell it from a host with a port. That is why net.JoinHostPort brackets,
	// and why everything the module builds goes through it.
	if err := HostWithOptionalPort("fd00::1:5001"); err != nil {
		t.Errorf("HostWithOptionalPort(\"fd00::1:5001\") is a bare IPv6 address and must be "+
			"accepted as one: %v", err)
	}
}

func TestHostWithOptionalPort(t *testing.T) {
	runRule(t, "HostWithOptionalPort", HostWithOptionalPort, []ruleCase{
		{in: "registry.example.com", accept: true, why: "a DNS name"},
		{in: "registry.d8-system.svc:5001", accept: true, why: "the in-cluster address the module generates"},
		{in: "127.0.0.1:5001", accept: true, why: "an IPv4 endpoint"},
		{in: "10.0.0.1", accept: true, why: "a bare IPv4 address"},
		{in: "fd00::1", accept: true, why: "a bare IPv6 address"},
		{in: "[fd00::1]:5001", accept: true, why: "a bracketed IPv6 endpoint"},
		{in: "registry.example.com.", accept: true, why: "the fully qualified form, which resolvers and container runtimes accept"},
		{in: "registry.example.com.:5001", accept: true, why: "the fully qualified form with a port"},
		{in: "registry.example.com..", why: "a second trailing dot is not a name"},
		{in: ".registry.example.com", why: "a leading dot is not a label"},
		{in: "-registry.example.com", why: "a leading hyphen reads as an option where the host becomes a command argument"},
		{in: "registry.example.com-", why: "a trailing hyphen is not a valid label"},
		{in: "a", accept: true, why: "a single-label host"},
		{in: "registry.example.com/path", why: "a path separator; this value becomes a directory name"},
		{in: "registry.example.com:0", why: "port 0"},
		{in: "registry.example.com:99999", why: "a port out of range"},
		{in: "registry.example.com:abc", why: "a port that is not a number"},
		{in: "registry.example.com:", why: "a trailing colon is malformed, not portless"},
		{in: ":5001", why: "no host"},
		{in: "-registry.example.com", why: "a DNS name may not start with a dash"},
		{in: "registry.example.com-", why: "nor end with one"},
		{in: "registry example.com", why: "a space would end a token in the generated files"},
		{in: "registry.example.com;x", why: "a separator of the sinks downstream"},
		{in: "$(id)", why: "a shell substitution, which an unquoted heredoc would run"},
		{in: "registry.example.com\nx", why: "a line break"},
		{in: "registry.example.com\x00", why: "a control character"},
	})
}

func TestIPWithPort(t *testing.T) {
	runRule(t, "IPWithPort", IPWithPort, []ruleCase{
		{in: "10.0.0.1:5001", accept: true, why: "the shape the module generates"},
		{in: "127.0.0.1:5001", accept: true, why: "the loopback balancer"},
		{in: "[fd00::1]:5001", accept: true, why: "IPv6 as net.JoinHostPort writes it"},
		{in: "0.0.0.0:1", accept: true, why: "the range boundaries are the port's, not the address's"},
		{in: "255.255.255.255:65535", accept: true, why: "the other boundary"},
		{in: "10.0.0.1", why: "a port is required: the NGINX server directive needs one"},
		{in: "fd00::1", why: "a bare IPv6 address is not an endpoint"},
		{in: "registry.example.com:5001", why: "a DNS name; endpoints come from InternalIP"},
		{in: "10.0.0.1:0", why: "port 0"},
		{in: "10.0.0.1:65536", why: "a port out of range"},
		{in: "10.0.0.1:-1", why: "a negative port"},
		{in: "10.0.0.1:abc", why: "a port that is not a number"},
		{in: "10.0.0.1:", why: "an empty port"},
		{in: ":5001", why: "no host"},
		{in: "999.999.999.999:5001", why: "not an address"},
		{in: "10.0.0.1:5001 ", why: "trailing space"},
		{in: "10.0.0.1:5001;x", why: "a value that would end the directive it is written into"},
		{in: NodeIPPlaceholder + ":5001", why: "the placeholder is ProxyEndpoint's business, not this rule's"},
		{in: "$(id):5001", why: "a shell substitution"},
		{in: "10.0.0.1:5001\n", why: "a line break"},
	})
}

func TestProxyEndpoint(t *testing.T) {
	runRule(t, "ProxyEndpoint", ProxyEndpoint, []ruleCase{
		{in: "10.0.0.1:5001", accept: true, why: "a master node's endpoint"},
		{in: "[fd00::1]:5001", accept: true, why: "the IPv6 form"},
		{in: NodeIPPlaceholder + ":5001", accept: true, why: "the bootstrap placeholder with a port"},
		{in: NodeIPPlaceholder, why: "the placeholder is only ever generated with a port"},
		{in: "${discovered_node_ip:-$(id)}:5001", why: "a near miss that would expand to a command"},
		{in: "${DISCOVERED_NODE_IP}:5001", why: "a different variable"},
		{in: "${discovered_node_ipX}:5001", why: "a longer name"},
		{in: NodeIPPlaceholder + "$(id):5001", why: "the placeholder with a substitution appended"},
		{in: NodeIPPlaceholder + NodeIPPlaceholder + ":5001", why: "the placeholder twice"},
		{in: NodeIPPlaceholder + ":0", why: "the placeholder with an invalid port"},
		{in: "10.0.0.1", why: "no port"},
		{in: "registry.example.com:5001", why: "a DNS name"},
		{in: "10.0.0.1:5001; return 200", why: "an NGINX directive appended"},
		{in: "10.0.0.1:5001;}\nserver { listen 127.0.0.1:5002; }", why: "a whole NGINX block"},
		{in: "10.0.0.1:5001 # comment", why: "an NGINX comment"},
		{in: "$(id > /tmp/pwned)", why: "a command substitution"},
		{in: "`id`", why: "the backtick form"},
		{in: "${IFS}", why: "a variable the shell would expand"},
		{in: "\\", why: "a backslash the shell would consume"},
		{in: "10.0.0.1:5001\x00", why: "a control character"},
	})
}

func TestMirrorHost(t *testing.T) {
	runRule(t, "MirrorHost", MirrorHost, []ruleCase{
		{in: "registry.d8-system.svc:5001", accept: true, why: "the in-cluster address"},
		{in: "127.0.0.1:5001", accept: true, why: "the node's own balancer"},
		{in: "registry.example.com", accept: true, why: "an upstream mirror"},
		{in: "fd00::1", accept: true, why: "a bare IPv6 address"},
		{in: NodeIPPlaceholder + ":5001", accept: true, why: "the bootstrap placeholder with a port"},
		{in: NodeIPPlaceholder, why: "the placeholder is only ever generated with a port"},
		{in: "${discovered_node_ip:-$(id)}:5001", why: "a near miss"},
		{in: "registry.example.com/path", why: "a path separator; this becomes a directory name"},
		{in: "../../../etc/cron.d/x", why: "a traversal into a directory the module does not own"},
		{in: "..", why: "the same, reduced"},
		{in: "registry.example.com:99999", why: "a port out of range"},
		{in: "registry.example.com\"]\n[host.\"http://evil\"", why: "a TOML table key reopened"},
		{in: "$(id)", why: "a command substitution"},
		{in: "`id`", why: "the backtick form"},
		{in: "registry.example.com\nx", why: "a line break"},
		{in: "registry.example.com\x00", why: "a control character"},
		{in: "registry example.com", why: "a space"},
		{in: "registry.example.com;x", why: "a separator"},
		{in: ":5001", why: "no host"},
		{in: "-registry.example.com", why: "a DNS name may not start with a dash"},
	})
}

func TestNodeIPPlaceholderWithPort(t *testing.T) {
	runRule(t, "NodeIPPlaceholderWithPort", NodeIPPlaceholderWithPort, []ruleCase{
		{in: NodeIPPlaceholder + ":5001", accept: true, why: "what dhctl writes"},
		{in: NodeIPPlaceholder + ":1", accept: true, why: "any valid port"},
		{in: NodeIPPlaceholder, why: "a port is required"},
		{in: NodeIPPlaceholder + ":", why: "an empty port"},
		{in: NodeIPPlaceholder + ":0", why: "port 0"},
		{in: NodeIPPlaceholder + ":abc", why: "a port that is not a number"},
		{in: "10.0.0.1:5001", why: "a real address is IPWithPort's business"},
		{in: "${discovered_node_ip:-$(id)}:5001", why: "a near miss"},
		{in: "${DISCOVERED_NODE_IP}:5001", why: "a different variable"},
		{in: "$discovered_node_ip:5001", why: "the unbraced form is not what dhctl writes"},
		{in: NodeIPPlaceholder + NodeIPPlaceholder + ":5001", why: "twice"},
		{in: " " + NodeIPPlaceholder + ":5001", why: "leading space"},
		{in: NodeIPPlaceholder + ":5001 ", why: "trailing space"},
		{in: strings.ToUpper(NodeIPPlaceholder) + ":5001", why: "the shell is case sensitive"},
		{in: "{discovered_node_ip}:5001", why: "no dollar"},
		{in: "$discovered_node_ip", why: "neither braces nor port"},
		{in: NodeIPPlaceholder + "/x:5001", why: "a path appended"},
		{in: NodeIPPlaceholder + ";x:5001", why: "a separator appended"},
		{in: "$(discovered_node_ip):5001", why: "a command substitution that looks similar"},
		{in: "``:5001", why: "an empty backtick substitution"},
	})
}

func TestRegistryAddress(t *testing.T) {
	runRule(t, "RegistryAddress", RegistryAddress, []ruleCase{
		{in: "registry.example.com/deckhouse/ee", accept: true, why: "the shape imagesRepo carries"},
		{in: "registry.example.com:5000/deckhouse/ee", accept: true, why: "with a port"},
		{in: "dev-registry.deckhouse.io/sys/deckhouse-oss", accept: true, why: "the address the stands use"},
		{in: "registry.example.com", accept: true, why: "a registry with no repository under it"},
		{in: "registry.example.com/", accept: true, why: "a trailing separator"},
		{in: "10.0.0.1:5000/x", accept: true, why: "an IP address"},
		{in: "[fd00::1]:5000/x", accept: true, why: "a bracketed IPv6 address"},
		{in: "../../../etc/cron.d/x", why: "the host would be \"..\", which reaches outside registry.d"},
		{in: "registry.example.com/a/../../b", why: "a traversal in the path"},
		{in: "registry.example.com/.", why: "a dot segment"},
		{in: "registry.example.com:0/x", why: "port 0"},
		{in: "registry.example.com:99999/x", why: "a port out of range"},
		{in: "/", why: "no host"},
		{in: "//x", why: "no host, with a path"},
		{in: "registry.example.com; return 200", why: "a separator of the sinks downstream"},
		{in: "registry.example.com$(id)", why: "a command substitution"},
		{in: "`id`/x", why: "the backtick form"},
		{in: "registry.example.com\nserver 127.0.0.1:5002", why: "a line break"},
		{in: "registry.example.com/x y", why: "a space in the path"},
		{in: "registry.example.com/x\x00", why: "a control character"},
	})
}

// TestEither pins the combinator the alternatives are built from.
func TestEither(t *testing.T) {
	accepts := validation.By(func(any) error { return nil })
	rejects := validation.By(func(any) error { return errFixture("no") })
	other := validation.By(func(any) error { return errFixture("nor this") })

	if err := either(accepts).Validate("x"); err != nil {
		t.Errorf("Either with one accepting rule must accept: %v", err)
	}
	if err := either(rejects, accepts).Validate("x"); err != nil {
		t.Errorf("Either must accept when any rule does: %v", err)
	}
	if err := either(accepts, rejects).Validate("x"); err != nil {
		t.Errorf("Either must not consult later rules once one accepts: %v", err)
	}

	err := either(rejects, other).Validate("x")
	if err == nil {
		t.Fatal("Either must reject when no rule accepts")
	}

	// The combined message has to name every alternative, because "not an
	// IP:port" alone would send a reader looking for the wrong mistake on a
	// value that was meant to be the bootstrap placeholder.
	for _, want := range []string{"no", "nor this"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Either's error %q must mention the failure %q", err, want)
		}
	}

	// No rules is vacuous rather than an error: a caller that built its rule
	// list from a loop must not be told the value is bad.
	if err := either().Validate("x"); err != nil {
		t.Errorf("Either with no rules must accept: %v", err)
	}
}

type errFixture string

func (e errFixture) Error() string { return string(e) }

func TestPortAndAccountName(t *testing.T) {
	runRule(t, "Port", Port, []ruleCase{
		{in: "5001", accept: true, why: "the module's port"},
		{in: "1", accept: true, why: "the lower boundary"},
		{in: "65535", accept: true, why: "the upper boundary"},
		{in: "0", why: "port 0"},
		{in: "65536", why: "one past the top"},
		{in: "-1", why: "negative"},
		{in: "abc", why: "not a number"},
		{in: "5001 ", why: "trailing space"},
		{in: " 5001", why: "leading space"},
		{in: "0x1", why: "not decimal"},
		{in: "+5001", why: "a sign; strconv.Atoi would take it but no consumer would"},
		{in: "5001.0", why: "not an integer"},
		{in: "99999999999999999999", why: "beyond int64"},
		{in: "٥", why: "a digit outside ASCII"},
		{in: "5001\n", why: "a line break"},
		{in: "50 01", why: "an inner space"},
		{in: "", accept: true, why: "absent"},
		{in: "00005001", accept: true, why: "leading zeros still parse to the same port"},
		{in: "5001;x", why: "a separator"},
		{in: "$(id)", why: "a substitution"},
	})

	long := strings.Repeat("a", maxAccountNameLength+1)
	runRule(t, "RegistryAccountName", RegistryAccountName, []ruleCase{
		{in: "ro", accept: true, why: "the read-only account"},
		{in: "rw", accept: true, why: "the read-write account"},
		{in: "mirror-puller", accept: true, why: "a generated name with a dash"},
		{in: "mirror_pusher", accept: true, why: "and with an underscore"},
		{in: "a.b", accept: true, why: "a dot inside"},
		{in: "a1", accept: true, why: "digits"},
		{in: strings.Repeat("a", maxAccountNameLength), accept: true, why: "the bound itself"},
		{in: long, why: "past the bound; a YAML simple key has a limit"},
		{in: "-ro", why: "may not start with a dash"},
		{in: "ro-", why: "nor end with one"},
		{in: ".ro", why: "nor start with a dot"},
		{in: "r o", why: "a space would end the key"},
		{in: "ro:", why: "a colon would end the key"},
		{in: "ro\"", why: "a quote would end a quoted key"},
		{in: "ro\n  x: y", why: "a line break adds a mapping entry"},
		{in: "ro\x00", why: "a control character"},
		{in: "$(id)", why: "a substitution"},
		{in: "ro/rw", why: "a path separator"},
		{in: "\u0440o", why: "a Cyrillic lookalike is outside the generated set"},
		{in: "ro#", why: "a comment marker"},
	})
}

func TestSchemeAddressAndProxy(t *testing.T) {
	runRule(t, "URLScheme", URLScheme, []ruleCase{
		{in: "http", accept: true, why: "one of the two"},
		{in: "https", accept: true, why: "and the other"},
		{in: "HTTP", why: "the consumers branch on the lowercase form"},
		{in: "HTTPS", why: "the same"},
		{in: "Https", why: "and mixed case"},
		{in: "ftp", why: "not a scheme the module supports"},
		{in: "http:", why: "a trailing colon"},
		{in: "http://", why: "a whole prefix"},
		{in: " http", why: "leading space"},
		{in: "http ", why: "trailing space"},
		{in: "https\nskip_verify = true", why: "a line break adds a TOML key"},
		{in: "https;x", why: "a separator"},
		{in: "$(id)", why: "a substitution"},
		{in: "h", why: "a prefix of one"},
		{in: "httpss", why: "a suffix of one"},
		{in: "tcp", why: "another protocol"},
		{in: "", accept: true, why: "absent"},
		{in: "http\x00", why: "a control character"},
		{in: "//", why: "a scheme-relative marker"},
		{in: "http?", why: "a query marker"},
	})

	runRule(t, "IPAddress", IPAddress, []ruleCase{
		{in: "10.0.0.1", accept: true, why: "an InternalIP"},
		{in: "0.0.0.0", accept: true, why: "the wildcard is still an address"},
		{in: "255.255.255.255", accept: true, why: "the top of the range"},
		{in: "fd00::1", accept: true, why: "IPv6"},
		{in: "::1", accept: true, why: "IPv6 loopback"},
		{in: "::", accept: true, why: "the IPv6 wildcard"},
		{in: "10.0.0.1:5001", why: "a port; this rule is for a bare address"},
		{in: "[fd00::1]", why: "brackets belong to the host:port form"},
		{in: "10.0.0.1/32", why: "a prefix length"},
		{in: "999.999.999.999", why: "not an address"},
		{in: "10.0.0", why: "too few octets"},
		{in: "10.0.0.1.2", why: "too many"},
		{in: "localhost", why: "a name, not an address"},
		{in: "registry.example.com", why: "the same"},
		{in: " 10.0.0.1", why: "leading space"},
		{in: "10.0.0.1 ", why: "trailing space"},
		{in: "10.0.0.1\n", why: "a line break"},
		{in: "$(id)", why: "a substitution"},
		{in: "", accept: true, why: "absent"},
		{in: "0x0a000001", why: "not dotted decimal"},
	})

	runRule(t, "ProxyURL", ProxyURL, []ruleCase{
		{in: "http://proxy.example.com:8080", accept: true, why: "the shape an operator sets"},
		{in: "https://proxy.example.com:8443", accept: true, why: "over TLS"},
		{in: "http://proxy.example.com", accept: true, why: "without a port"},
		{in: "http://10.0.0.1:8080", accept: true, why: "an address"},
		{in: "proxy.example.com:8080", why: "no scheme"},
		{in: "//proxy.example.com", why: "a scheme-relative URL has no scheme"},
		{in: "http://", why: "no host"},
		{in: "ftp://proxy.example.com", why: "a scheme the module does not support"},
		{in: "http://proxy.example.com/path", why: "a path the module has no use for"},
		{in: "http://proxy.example.com?q=1", why: "a query"},
		{in: "http://proxy.example.com#f", why: "a fragment"},
		// Credentials are part of the documented shape, not an abuse of it:
		// cluster_configuration.yaml gives these very forms as the examples for
		// proxy.httpProxy, including the percent-encoded ones a Windows domain
		// account and an account with an "@" in it require.
		{in: "http://user:pass@proxy.example.com", accept: true, why: "credentials, which the schema documents"},
		{in: "https://user:password@proxy.company.my:8443", accept: true, why: "the schema's own example"},
		{in: "https://DOMAIN%5Cuser:password@proxy.company.my:8443", accept: true, why: "a Windows domain account, percent-encoded"},
		{in: "https://user%40domain.local:password@proxy.company.my:8443", accept: true, why: "an account carrying an @, percent-encoded"},
		{in: "https://user@proxy.company.my", accept: true, why: "a user with no password"},
		{in: "https://user:p%40ss:word@proxy.company.my:8443", accept: true, why: "a colon in the password, which the schema allows and net/url would re-encode"},
		{in: "http://user:pass@proxy.example.com/path", why: "credentials do not license a path"},
		{in: "http://user:pass@proxy.example.com#f", why: "credentials do not license a fragment"},
		{in: "http://0#", why: "a fragment the component checks alone would miss"},
		{in: "http://proxy.example.com:99999", why: "a port out of range"},
		{in: "http://proxy.example.com:abc", why: "a port that is not a number"},
		// A scheme is case-insensitive per RFC 3986 and net/url normalises it,
		// so this names the same proxy. It was refused only as a side effect of
		// the canonical-form comparison that had to go.
		{in: "HTTP://proxy.example.com", accept: true, why: "an upper-case scheme names the same proxy"},
		{in: "http://proxy.example.com\n", why: "a line break"},
		{in: "$(id)", why: "a substitution"},
		{in: "", accept: true, why: "a proxy that is not configured"},
		{in: "http://proxy.example.com:8080/", accept: true, why: "a bare / is the empty path; every client treats the two alike"},
	})

	runRule(t, "NoProxyList", NoProxyList, []ruleCase{
		{in: "localhost,127.0.0.1", accept: true, why: "the shape an operator sets"},
		{in: "localhost", accept: true, why: "a single entry"},
		{in: ".example.com", accept: true, why: "a domain suffix"},
		{in: "10.0.0.0/8", accept: true, why: "a CIDR block"},
		{in: "*", accept: true, why: "the wildcard"},
		{in: "*.example.com", accept: true, why: "a wildcard suffix"},
		{in: "registry.d8-system.svc:5001", accept: true, why: "a host with a port"},
		{in: "example.com/", accept: true, why: "a trailing slash, which the noProxy schema admits"},
		{in: "10.0.0.0/8/", accept: true, why: "a CIDR with a trailing slash, likewise"},
		{in: "/", why: "a slash alone is not an entry"},
		{in: "..", why: "not a host, a suffix or a block"},
		{in: "localhost,,127.0.0.1", why: "an empty entry"},
		{in: ",", why: "nothing but a separator"},
		{in: "local host", why: "a space inside an entry"},
		{in: "localhost, 127.0.0.1", accept: true, why: "entries are trimmed, as every no_proxy consumer does"},
		{in: "localhost;127.0.0.1", why: "the wrong separator"},
		{in: "localhost\n127.0.0.1", why: "a line break"},
		{in: "$(id)", why: "a substitution"},
		{in: "`id`", why: "the backtick form"},
		{in: "localhost\x00", why: "a control character"},
		{in: "", accept: true, why: "absent"},
		{in: "-localhost", why: "an entry may not start with a dash"},
		{in: "localhost-", why: "nor end with one"},
		{in: "localhost,", why: "a trailing separator leaves an empty entry"},
	})

	runRule(t, "EncodableString", EncodableString, []ruleCase{
		{in: "ordinary", accept: true, why: "plain text"},
		{in: "with spaces", accept: true, why: "spaces are not the encoder's problem"},
		{in: "with\ttab", why: "a tab is refused with the other control characters; nothing generated carries one"},
		{in: "üñïçødé", accept: true, why: "valid UTF-8 outside ASCII"},
		{in: "$(id)", accept: true, why: "shell metacharacters are the sinks' problem, not the encoder's"},
		{in: "a\nb", why: "a line break would add a line to a generated file"},
		{in: "a\rb", why: "and a carriage return"},
		{in: "a\x00b", why: "a NUL has no representation"},
		{in: "a\x01b", why: "nor another control character"},
		{in: "a\x7fb", why: "nor delete"},
		{in: "a\x1bb", why: "nor escape"},
		{in: "\xff", why: "not valid UTF-8: the encoder would rewrite it"},
		{in: "\xe3", why: "a truncated sequence"},
		{in: "a\xffb", why: "an invalid byte inside valid text"},
		{in: "\xed\xa0\x80", why: "a surrogate half"},
		{in: "", accept: true, why: "absent"},
		{in: strings.Repeat("a", 10000), accept: true, why: "length is not what this rule bounds"},
		{in: " ", accept: true, why: "a non-breaking space is a valid rune"},
		{in: "a b", accept: true, why: "a Unicode line separator is not a YAML line break"},
		{in: "\ufeff", accept: true, why: "a byte order mark is a valid rune"},
	})
}
