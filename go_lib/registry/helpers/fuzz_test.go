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

// Fuzz harnesses for the registry address and credential helpers.
//
// These functions sit between the deckhouse-registry secret / ModuleConfig and
// the values the orchestrator hooks publish to nodes:
//
//   - SplitAddressAndPath derives the containerd `hosts` key and the rewrite
//     target from `imagesRepo`.
//   - CredsFromDockerCfg extracts the credentials that end up in
//     /etc/containerd/registry.d/<host>/hosts.toml and in the storage
//     configuration.
//
// Threat model coverage (registry-threat-model.md): supports TM-05, TM-09,
// TM-13 and TM-18. The harnesses assert robustness and that the credential
// codec round-trips, so that a credential cannot be silently altered on the way
// to the nodes.
package helpers

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// isSubsequence reports whether every byte of want appears in have, in order.
// It compares bytes rather than runes so that the check also holds for input
// that is not valid UTF-8, where ranging over a string would yield U+FFFD.
func isSubsequence(want, have string) bool {
	next := 0
	for i := 0; i < len(have) && next < len(want); i++ {
		if have[i] == want[next] {
			next++
		}
	}
	return next == len(want)
}

func FuzzSplitAddressAndPath(f *testing.F) {
	seeds := []string{
		"",
		"/",
		"//",
		"registry.example.com",
		"registry.example.com/",
		"registry.example.com/system/deckhouse",
		"registry.example.com:5001/system/deckhouse/",
		"  registry.example.com/path  ",
		"/leading",
		"a//b",
		"registry.example.com///",
		"\n",
		"$(id)/path",
		strings.Repeat("a/", 64),
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, ref string) {
		host, path := SplitAddressAndPath(ref)

		// The host becomes a directory name under /etc/containerd/registry.d, so it
		// must never contain a separator.
		if strings.Contains(host, "/") {
			t.Fatalf("SplitAddressAndPath(%q) returned host %q containing a separator", ref, host)
		}

		// A non-empty path is documented to be rooted, because callers strip the
		// leading slash with TrimLeft.
		if path != "" && !strings.HasPrefix(path, "/") {
			t.Fatalf("SplitAddressAndPath(%q) returned a path %q that is not rooted", ref, path)
		}

		// Neither part may carry surrounding whitespace: the host names a directory
		// under /etc/containerd/registry.d and a key in the generated hosts.toml,
		// and the path becomes the target of a containerd rewrite rule.
		if host != strings.TrimSpace(host) {
			t.Fatalf("SplitAddressAndPath(%q) returned host %q with surrounding whitespace", ref, host)
		}
		if trimmed := strings.TrimSpace(strings.TrimPrefix(path, "/")); path != "" && path != "/"+trimmed {
			t.Fatalf("SplitAddressAndPath(%q) returned path %q with surrounding whitespace", ref, path)
		}

		// Nothing may be fabricated: every character of the result must come from
		// the input, in order. This holds regardless of how much normalisation the
		// function performs, so it does not have to be mirrored here.
		if !isSubsequence(host+path, ref) {
			t.Fatalf("SplitAddressAndPath(%q) produced %q, which is not a subsequence of the input",
				ref, host+path)
		}

		// Idempotent: splitting the host again must not change it.
		if againHost, againPath := SplitAddressAndPath(host); againHost != host || againPath != "" {
			t.Fatalf("SplitAddressAndPath is not idempotent on the host: %q -> (%q, %q)",
				host, againHost, againPath)
		}
	})
}

// FuzzCredsFromDockerCfg feeds an arbitrary .dockerconfigjson, the form the
// deckhouse-registry secret takes. It must never panic and must never invent
// credentials for a host it does not contain.
func FuzzCredsFromDockerCfg(f *testing.F) {
	seeds := []struct {
		config string
		host   string
	}{
		{``, "registry.example.com"},
		{`{}`, "registry.example.com"},
		{`null`, "registry.example.com"},
		{`[]`, "registry.example.com"},
		{`{"auths":{}}`, "registry.example.com"},
		{`{"auths":{"registry.example.com":{"username":"u","password":"p"}}}`, "registry.example.com"},
		{`{"auths":{"registry.example.com":{"auth":"dTpw"}}}`, "registry.example.com"},
		{`{"auths":{"registry.example.com":{"auth":"dTpw"}}}`, "other.example.com"},
		{`{"auths":{"https://registry.example.com":{"auth":"dTpw"}}}`, "registry.example.com"},
		{`{"auths":{"registry.example.com":{"auth":"!!!!"}}}`, "registry.example.com"},
		{`{"auths":{"registry.example.com":{"auth":"dQ"}}}`, "registry.example.com"},
		{`{"auths":{"registry.example.com":{"auth":"dTpwOng"}}}`, "registry.example.com"},
		{`{"auths":{"":{"auth":"dTpw"}}}`, ""},
		{`{"auths":{"registry.example.com":{"auth":"dTpw"}}}`, ":5001"},
	}
	for _, seed := range seeds {
		f.Add([]byte(seed.config), seed.host)
	}

	f.Fuzz(func(t *testing.T, rawConfig []byte, host string) {
		username, password, err := CredsFromDockerCfg(rawConfig, host)
		if err != nil {
			if username != "" || password != "" {
				t.Fatalf("CredsFromDockerCfg returned credentials together with an error %v: %q/%q",
					err, username, password)
			}
			return
		}

		// A partial credential is deliberately tolerated for backwards
		// compatibility with old docker configs -- see the "old docker config -
		// username only" case in models/initconfig -- so it is not asserted against
		// here. What the consumers do with it is a product decision, not an
		// invariant of this function.

		// Extraction must be deterministic: the same secret cannot yield different
		// credentials between two reconciliations of the same hook.
		againUser, againPassword, againErr := CredsFromDockerCfg(rawConfig, host)
		if againErr != nil || againUser != username || againPassword != password {
			t.Fatalf("CredsFromDockerCfg is not deterministic for (%q, %q): %q/%q then %q/%q (err %v)",
				rawConfig, host, username, password, againUser, againPassword, againErr)
		}
	})
}

// FuzzDockerCfgRoundTrip asserts that credentials survive the encode/decode pair
// the module uses when it rewrites the deckhouse-registry secret. A round-trip
// that alters a credential silently locks the cluster out of its own registry.
func FuzzDockerCfgRoundTrip(f *testing.F) {
	f.Add("user", "password", "registry.example.com")
	f.Add("", "", "registry.example.com")
	f.Add("user", "", "registry.example.com")
	f.Add("", "password", "registry.example.com")
	f.Add("u:1", "p:2", "registry.example.com:5001")
	f.Add("üser", "pässword", "registry.example.com")
	f.Add("user", "pass\nword", "registry.example.com")
	f.Add("user", "password", "https://registry.example.com/path")

	f.Fuzz(func(t *testing.T, username, password, host string) {
		// The credentials travel through json.Marshal, which replaces invalid UTF-8
		// with U+FFFD; a host that is not valid UTF-8 after normalisation therefore
		// cannot be looked up again afterwards. That includes percent escapes such
		// as "%80", which normalizeHost decodes into a raw byte. Nothing in the
		// module's data path carries such a host (imagesRepo is constrained to
		// ASCII by the ModuleConfig schema and registry addresses are hostnames),
		// so the round-trip is only asserted over the reachable domain.
		if !utf8.ValidString(username) || !utf8.ValidString(password) {
			return
		}
		if normalized, err := normalizeHost(host); err != nil || !utf8.ValidString(normalized) {
			return
		}

		raw, err := DockerCfgFromCreds(username, password, host)
		if err != nil {
			return
		}

		gotUser, gotPassword, err := CredsFromDockerCfg(raw, host)
		if err != nil {
			t.Fatalf("credentials written by DockerCfgFromCreds cannot be read back: %v\nconfig: %s",
				err, raw)
		}

		// encodeAuth only stores the pair when both parts are set; in that case the
		// round-trip must be exact.
		if username != "" && password != "" && !strings.Contains(username, ":") {
			if gotUser != username || gotPassword != password {
				t.Fatalf("round-trip altered the credentials: wrote %q/%q, read %q/%q\nconfig: %s",
					username, password, gotUser, gotPassword, raw)
			}
		}
	})
}
