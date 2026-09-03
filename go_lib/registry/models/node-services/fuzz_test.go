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

// Fuzz harnesses for the node services configuration model.
//
// This model is the payload of the d8-system/registry-node-config-<node> secret.
// It is untrusted input for a privileged component: registry-nodeservices-manager
// runs as root on control plane nodes, mounts /etc/kubernetes read-write and
// renders this configuration into a static pod manifest that kubelet executes.
//
// Threat model coverage (registry-threat-model.md):
//
//   - TM-02 / AS-02: `ProxyConfig.Validate()` unconditionally returns nil, so no
//     constraint is placed on values that are later interpolated into the static
//     pod manifest without escaping.
//   - TM-08 / AS-08: `LocalMode.Upstreams` and `ProxyMode.Upstream` are not
//     checked for format before being interpolated into the mirrorer and
//     distribution configurations.
//   - TM-01 / TM-07: Validate() feeds arbitrary strings into the PKI decoding and
//     CA chain verification routines.
package nodeservices

import (
	"encoding/json"
	"net"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// unsafeInConfigFile are characters that let a value escape the YAML scalar or
// the shell context it is interpolated into by the rendering templates.
const unsafeInConfigFile = "\"'\n\r$`\\{}#"

var injectionSeeds = []string{
	"",
	" ",
	"\n",
	"\r\n",
	"\t",
	"https",
	"http",
	"HTTPS",
	"ftp",
	"registry.example.com",
	"10.0.0.1",
	"10.0.0.1:5001",
	`"`,
	`x"` + "\n" + `remote: ["evil:5001"]`,
	"x\n      - name: INJECTED\n        value: pwned",
	"$(id)",
	"`id`",
	"a#b",
	"a{b}",
	"---",
	"null",
	"true",
	"0700",
}

// assertRenderSafe fails if a value accepted by Validate() could alter the
// structure of a rendered configuration file.
func assertRenderSafe(t *testing.T, field, value string) {
	t.Helper()

	if i := strings.IndexAny(value, unsafeInConfigFile); i >= 0 {
		t.Fatalf("%s = %q was accepted by Validate() but contains %q at offset %d, "+
			"which can escape the scalar it is rendered into", field, value, value[i], i)
	}
}

// ---------------------------------------------------------------------------
// TM-01 / TM-07: robustness of Validate() against an arbitrary secret payload.
// ---------------------------------------------------------------------------

// FuzzConfigJSONValidate decodes arbitrary JSON as the node configuration secret
// and validates it. Validate() parses PEM certificates and keys and verifies a CA
// chain, so it must survive any byte sequence without panicking: a panic here
// crashes the controller and stops node services from being reconciled.
func FuzzConfigJSONValidate(f *testing.F) {
	seeds := []string{
		`{}`,
		`null`,
		`[]`,
		`{"ca":"","auth_cert":"","auth_key":""}`,
		`{"ca":"-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----"}`,
		`{"ca":"-----BEGIN CERTIFICATE-----","auth_cert":"-----BEGIN CERTIFICATE-----"}`,
		`{"ca":"x","auth_cert":"x","auth_key":"x","token_cert":"x","token_key":"x",` +
			`"distribution_cert":"x","distribution_key":"x","http_secret":"x",` +
			`"user_ro":{"name":"ro","password":"p","password_hash":"h"}}`,
		`{"local_mode":{"upstreams":["$(id)"]},"proxy_mode":{"upstream":{}}}`,
		`{"proxy_config":{"http":"x\ny","https":"","no_proxy":""}}`,
		`{"proxy_mode":{"upstream":{"scheme":"https","host":"h","path":"/p","ttl":"5m"}}}`,
		`{"local_mode":{"user_rw":{"name":"n","password":"p","password_hash":"h"}}}`,
	}
	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, raw []byte) {
		var config Config
		if err := json.Unmarshal(raw, &config); err != nil {
			return
		}

		// Must not panic, whatever the payload.
		_ = config.Validate()

		// Validate() must be a pure predicate: repeating it cannot change the answer.
		first := config.Validate() == nil
		if second := config.Validate() == nil; first != second {
			t.Fatalf("Validate() is not deterministic for input: %s", raw)
		}

		// A configuration that validates must name exactly one mode, because the
		// renderer branches on that and would otherwise emit a manifest for a mode
		// the operator did not select.
		if first {
			if (config.LocalMode == nil) == (config.ProxyMode == nil) {
				t.Fatalf("Validate() accepted a config with local_mode=%v proxy_mode=%v: %s",
					config.LocalMode != nil, config.ProxyMode != nil, raw)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// TM-02: ProxyConfig has no validation at all.
// ---------------------------------------------------------------------------

// FuzzProxyConfigValidate asserts that a proxy configuration accepted by
// Validate() can be rendered into the static pod manifest without altering it.
// The values become HTTP_PROXY/HTTPS_PROXY/NO_PROXY environment variables in a
// manifest that kubelet runs as root on a control plane node.
func FuzzProxyConfigValidate(f *testing.F) {
	for _, seed := range injectionSeeds {
		f.Add(seed, "https://proxy.example.com:8443", "localhost,127.0.0.1")
		f.Add("http://proxy.example.com:8080", seed, "localhost,127.0.0.1")
		f.Add("http://proxy.example.com:8080", "https://proxy.example.com:8443", seed)
	}

	f.Fuzz(func(t *testing.T, proxyHTTP, proxyHTTPS, noProxy string) {
		config := ProxyConfig{
			HTTP:    proxyHTTP,
			HTTPS:   proxyHTTPS,
			NoProxy: noProxy,
		}

		if err := config.Validate(); err != nil {
			return
		}

		assertRenderSafe(t, "proxy_config.http", config.HTTP)
		assertRenderSafe(t, "proxy_config.https", config.HTTPS)
		assertRenderSafe(t, "proxy_config.no_proxy", config.NoProxy)

		// HTTP_PROXY and HTTPS_PROXY are consumed by Go's net/http proxy support,
		// so an accepted value must be a URL it can use.
		for field, value := range map[string]string{
			"proxy_config.http":  config.HTTP,
			"proxy_config.https": config.HTTPS,
		} {
			if value == "" {
				continue
			}
			parsed, err := url.Parse(value)
			if err != nil {
				t.Fatalf("%s = %q was accepted by Validate() but is not a URL: %v", field, value, err)
			}
			if parsed.Scheme != "http" && parsed.Scheme != "https" {
				t.Fatalf("%s = %q was accepted by Validate() but has scheme %q", field, value, parsed.Scheme)
			}
			if parsed.Host == "" {
				t.Fatalf("%s = %q was accepted by Validate() but has no host", field, value)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// TM-08: LocalMode.Upstreams reaches the mirrorer config unescaped.
// ---------------------------------------------------------------------------

// FuzzLocalModeUpstreams asserts that mirror targets accepted by Validate() are
// the plain addresses the mirrorer template expects. They decide which replicas
// images are pushed to, so an unconstrained value redirects mirroring.
func FuzzLocalModeUpstreams(f *testing.F) {
	for _, seed := range injectionSeeds {
		f.Add(seed, "10.0.0.2")
		f.Add("10.0.0.1", seed)
	}

	user := User{Name: "u", Password: "p", PasswordHash: "h"}

	f.Fuzz(func(t *testing.T, first, second string) {
		mode := LocalMode{
			UserRW:     user,
			UserPuller: user,
			UserPusher: user,
			Upstreams:  []string{first, second},
		}

		if err := mode.Validate(); err != nil {
			return
		}

		for i, upstream := range mode.Upstreams {
			field := "local_mode.upstreams[" + strconv.Itoa(i) + "]"
			assertRenderSafe(t, field, upstream)

			if upstream == "" {
				t.Fatalf("%s is empty but was accepted by Validate(); it renders as \":5001\"", field)
			}
			if net.ParseIP(upstream) == nil {
				t.Fatalf("%s = %q was accepted by Validate() but is not an IP address; "+
					"mirror targets are built from the InternalIP of control plane nodes", field, upstream)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// TM-06 / TM-08: upstream registry parameters reach the distribution config.
// ---------------------------------------------------------------------------

// FuzzUpstreamRegistryValidate asserts that an accepted upstream registry can be
// rendered into `remoteurl: "<scheme>://<host>"` without breaking out of the
// scalar, and that it points somewhere the module can actually reach.
func FuzzUpstreamRegistryValidate(f *testing.F) {
	for _, seed := range injectionSeeds {
		f.Add(seed, "upstream.example.com", "/system/deckhouse", "user", "pass")
		f.Add("https", seed, "/system/deckhouse", "user", "pass")
		f.Add("https", "upstream.example.com", seed, "user", "pass")
		f.Add("https", "upstream.example.com", "/system/deckhouse", seed, "pass")
		f.Add("https", "upstream.example.com", "/system/deckhouse", "user", seed)
	}

	f.Fuzz(func(t *testing.T, scheme, host, path, user, password string) {
		upstream := UpstreamRegistry{
			Scheme:   scheme,
			Host:     host,
			Path:     path,
			User:     user,
			Password: password,
		}

		if err := upstream.Validate(); err != nil {
			return
		}

		assertRenderSafe(t, "upstream.scheme", upstream.Scheme)
		assertRenderSafe(t, "upstream.host", upstream.Host)
		assertRenderSafe(t, "upstream.path", upstream.Path)

		// The scheme is concatenated into a URL, so only the two the module supports
		// may be accepted.
		if upstream.Scheme != "http" && upstream.Scheme != "https" {
			t.Fatalf("upstream.scheme = %q was accepted by Validate() but is neither http nor https",
				upstream.Scheme)
		}

		// `<scheme>://<host>` must parse back to the same host: otherwise the value
		// the module believes it configured is not the one distribution will use.
		parsed, err := url.Parse(upstream.Scheme + "://" + upstream.Host)
		if err != nil {
			t.Fatalf("upstream %q://%q was accepted by Validate() but is not a URL: %v",
				upstream.Scheme, upstream.Host, err)
		}
		if parsed.Host != upstream.Host {
			t.Fatalf("upstream.host = %q was accepted by Validate() but parses back as %q",
				upstream.Host, parsed.Host)
		}
		if parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
			t.Fatalf("upstream.host = %q was accepted by Validate() but carries extra URL components "+
				"(path=%q query=%q fragment=%q userinfo=%v)",
				upstream.Host, parsed.Path, parsed.RawQuery, parsed.Fragment, parsed.User)
		}
	})
}
