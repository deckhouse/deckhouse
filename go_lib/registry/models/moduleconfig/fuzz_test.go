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

// Fuzz harnesses for the upstream registry parameters as they enter the module.
//
// Threat model coverage (registry-threat-model.md), harness 2, input side: the
// document asks for robustness against malformed values of the upstream
// registry's address, scheme, path and CA, as they are substituted into the
// service configurations. This is where those four values arrive -- the
// ModuleConfig the operator writes, and the deckhouse-registry secret whose
// contents reach the same fields.
//
// The values do not stop here. `imagesRepo` is split into a host and a path by
// helpers.SplitAddressAndPath and both halves reach:
//
//   - the `remoteurl` of the distribution configuration,
//   - the mirror host in /etc/containerd/registry.d/<host>/hosts.toml, written
//     through an unquoted heredoc that runs as root,
//   - `imagesBase`, which every node pulls from.
//
// So the oracle is the one used throughout this module: anything Validate()
// accepts must be safe for every sink it reaches. imagesRepoRegexp is what makes
// that true today, and these harnesses hold it to that -- a widening of the
// pattern that let a separator through would show up here rather than on a node.

package moduleconfig

import (
	"encoding/json"
	"strings"
	"testing"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	"github.com/deckhouse/deckhouse/go_lib/registry/helpers"
)

// shellMetaCharacters are expanded by bash inside the unquoted heredoc that
// writes hosts.toml, whatever quoting the template applies to the value.
const shellMetaCharacters = "$`\\"

// tomlNGINXMetaCharacters would terminate or reopen a TOML table key or an NGINX
// directive built from the value.
const tomlNGINXMetaCharacters = ";{}#'\"[] \t\r\n"

func FuzzRegistrySettingsValidate(f *testing.F) {
	// The settings an operator writes, then each of the five fields taken in
	// turn to where it stops being one: an address with no host, a traversing
	// path, an impossible port, a scheme that is neither, a CA against HTTP,
	// and credentials given one half at a time.
	f.Add("registry.example.com/deckhouse/ee", "HTTPS", "", "", "")
	f.Add("registry.example.com:5000/deckhouse/ee", "HTTPS", "", "user", "password")
	f.Add("dev-registry.deckhouse.io/sys/deckhouse-oss", "HTTPS", "", "", "")
	f.Add("registry.example.com", "HTTP", "", "", "")
	f.Add("registry.example.com/", "HTTPS", "", "", "")
	f.Add("", "HTTPS", "", "", "")
	f.Add("/", "HTTPS", "", "", "")
	f.Add("../../../etc/cron.d/x", "HTTPS", "", "", "")
	f.Add("registry.example.com/a/../../b", "HTTPS", "", "", "")
	f.Add("registry.example.com:0/x", "HTTPS", "", "", "")
	f.Add("registry.example.com:99999/x", "HTTPS", "", "", "")
	f.Add("registry.example.com; return 200", "HTTPS", "", "", "")
	f.Add("registry.example.com$(id)", "HTTPS", "", "", "")
	f.Add("[fd00::1]:5000/x", "HTTPS", "", "", "")
	f.Add("registry.example.com/x", "https", "", "", "")
	f.Add("registry.example.com/x", "", "", "", "")
	f.Add("registry.example.com/x", "ftp", "", "", "")
	f.Add("registry.example.com/x", "HTTP", "ca", "", "")
	f.Add("registry.example.com/x", "HTTPS", "-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----\n", "", "")
	f.Add("registry.example.com/x", "HTTPS", "", "user", "")

	f.Fuzz(func(t *testing.T, imagesRepo, scheme, ca, username, password string) {
		if len(imagesRepo) > 8192 || len(scheme) > 64 || len(ca) > 8192 {
			return
		}

		settings := RegistrySettings{
			ImagesRepo: imagesRepo,
			Scheme:     constant.SchemeType(scheme),
			CA:         ca,
			Username:   username,
			Password:   password,
		}

		err := settings.Validate()

		// Validation must be a decision, not a coin flip: the hook calls it on
		// one copy and the consumer on another.
		if second := settings.Validate(); (err == nil) != (second == nil) {
			t.Fatalf("Validate() is not deterministic for imagesRepo %q: %v then %v",
				imagesRepo, err, second)
		}

		if err != nil {
			return
		}

		// The scheme selects TLS handling in every consumer.
		if settings.Scheme != constant.SchemeHTTP && settings.Scheme != constant.SchemeHTTPS {
			t.Fatalf("Validate() accepted scheme %q, which is neither %q nor %q",
				settings.Scheme, constant.SchemeHTTP, constant.SchemeHTTPS)
		}

		// The split is what the module performs downstream, so the oracle looks
		// at the halves that actually reach the sinks.
		host, path := helpers.SplitAddressAndPath(imagesRepo)

		if hostErr := helpers.HostWithOptionalPort(host); hostErr != nil {
			t.Fatalf("Validate() accepted imagesRepo %q, whose host %q is not a registry host: %v; "+
				"that host becomes a directory name under /etc/containerd/registry.d and a "+
				"table key in hosts.toml", imagesRepo, host, hostErr)
		}
		if pathErr := helpers.URLPath(path); pathErr != nil {
			t.Fatalf("Validate() accepted imagesRepo %q, whose path %q is not a repository path: %v",
				imagesRepo, path, pathErr)
		}

		for name, value := range map[string]string{"host": host, "path": path} {
			if i := strings.IndexAny(value, shellMetaCharacters); i >= 0 {
				t.Fatalf("Validate() accepted imagesRepo %q, whose %s %q carries the shell "+
					"metacharacter %q at offset %d; it is interpolated into an unquoted heredoc "+
					"that runs as root on every node", imagesRepo, name, value, value[i], i)
			}
			if i := strings.IndexAny(value, tomlNGINXMetaCharacters); i >= 0 {
				t.Fatalf("Validate() accepted imagesRepo %q, whose %s %q carries %q at offset %d, "+
					"which can close or reopen a TOML table key in hosts.toml",
					imagesRepo, name, value, value[i], i)
			}
		}

		// Credentials travel as a pair; one without the other is a
		// half-configured pull that fails on the node.
		if (username == "") != (password == "") {
			t.Fatalf("Validate() accepted username %q with password %q; the two must be "+
				"present or absent together", username, password)
		}
	})
}

// FuzzDeckhouseSettingsJSON drives the whole document the way the module reads
// it: from JSON, then validated, then rendered to the map the values carry.
func FuzzDeckhouseSettingsJSON(f *testing.F) {
	f.Add(`{"mode":"Direct","direct":{"imagesRepo":"registry.example.com/x","scheme":"HTTPS"}}`)
	f.Add(`{"mode":"Proxy","proxy":{"imagesRepo":"registry.example.com/x","scheme":"HTTPS","ttl":"1h"}}`)
	f.Add(`{"mode":"Unmanaged","unmanaged":{"imagesRepo":"registry.example.com/x","scheme":"HTTP"}}`)
	f.Add(`{"mode":"Local"}`)
	f.Add(`{"mode":"Direct"}`)
	f.Add(`{"mode":"Direct","proxy":{"imagesRepo":"x","scheme":"HTTPS"}}`)
	f.Add(`{"mode":"Unknown"}`)
	f.Add(`{}`)
	f.Add(`{"mode":"Direct","direct":null}`)
	f.Add(`{"MODE":"Direct"}`)
	f.Add(`{"mode":"Direct","direct":{"imagesRepo":"registry.example.com/x","scheme":"HTTPS","license":"l","username":"u","password":"p"}}`)
	f.Add(`{"mode":"Proxy","proxy":{"imagesRepo":"registry.example.com/x","scheme":"HTTPS","ttl":"1s"}}`)
	f.Add(`{`)
	f.Add(``)
	f.Add(`null`)
	f.Add(`[]`)
	f.Add(`{"mode":"Local","direct":{"imagesRepo":"registry.example.com/x","scheme":"HTTPS"}}`)
	f.Add(`{"mode":"Direct","direct":{"imagesRepo":"registry.example.com/x","scheme":"HTTPS"},"unmanaged":{"imagesRepo":"y","scheme":"HTTP"}}`)
	f.Add(`{"mode":"Proxy","proxy":{"imagesRepo":"registry.example.com/x","scheme":"HTTPS","ttl":"$(id)"}}`)
	f.Add(`{"mode":"Unmanaged","unmanaged":{"imagesRepo":"../../etc","scheme":"HTTPS"}}`)

	f.Fuzz(func(t *testing.T, document string) {
		if len(document) > 1<<16 {
			return
		}

		var settings DeckhouseSettings
		if err := json.Unmarshal([]byte(document), &settings); err != nil {
			return
		}

		err := settings.Validate()

		// ToMap feeds the module values and must not depend on the outcome of
		// validation; it must simply not panic on anything that parsed.
		_ = settings.ToMap()

		// DeepCopy must be independent: a caller may keep one copy and mutate
		// another.
		clone := settings.DeepCopy()
		if clone == nil {
			t.Fatal("DeepCopy() returned nil for a parsed document")
		}
		if (clone.Validate() == nil) != (err == nil) {
			t.Fatalf("DeepCopy() changed the validity of %q: %v then %v",
				document, err, clone.Validate())
		}

		// Merge(nil) is the identity, which is what makes the merge order in the
		// hook safe to reason about.
		merged := settings.Merge(nil)
		if (merged.Validate() == nil) != (err == nil) {
			t.Fatalf("Merge(nil) changed the validity of %q", document)
		}
		if merged.Mode != settings.Mode {
			t.Fatalf("Merge(nil) changed the mode of %q from %q to %q",
				document, settings.Mode, merged.Mode)
		}

		if err != nil {
			return
		}

		// Exactly one section may be populated, and it must be the one the mode
		// names: the hook switches on the section, so a second one would be a
		// configuration that means two different things.
		sections := 0
		for _, populated := range []bool{settings.Direct != nil, settings.Unmanaged != nil} {
			if populated {
				sections++
			}
		}
		switch settings.Mode {
		case constant.ModeLocal:
			if sections != 0 {
				t.Fatalf("Validate() accepted mode Local with %d populated sections: %q",
					sections, document)
			}
		default:
			if sections != 1 {
				t.Fatalf("Validate() accepted mode %q with %d populated sections: %q",
					settings.Mode, sections, document)
			}
		}
	})
}
