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

// Fuzz harnesses for the orchestrator's own view of the upstream parameters.
//
// Threat model coverage (registry-threat-model.md), harness 2, input side. The
// ModuleConfig boundary is covered by go_lib/registry/models/moduleconfig; this
// is the second door into the same fields, and the one that is not a
// ModuleConfig: ParamsState is JSON persisted in the module values and read
// back, so a value can re-enter here without passing the ModuleConfig schema
// again. The CA travels through it as PEM and is decoded on the way back.
//
// The invariants are about the state machine rather than about sinks, because
// the sinks are reached from bashible.ConfigBuilder and the service config
// templates, which have harnesses of their own:
//
//   - Reading back persisted state must not panic, whatever it holds.
//   - The round trip must not lose or alter a field. The orchestrator compares
//     the state it read with the parameters it computed to decide whether a
//     registry transition is needed; a field that changes in transit is either
//     a transition that never fires or one that fires forever.
//   - Validate() must be a decision, not a coin flip.

package orchestrator

import (
	"crypto/x509"
	"encoding/json"
	"testing"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	registry_pki "github.com/deckhouse/deckhouse/go_lib/registry/pki"
)

func FuzzParamsStateRoundTrip(f *testing.F) {
	f.Add(`{"generation":1,"mode":"Direct","images_repo":"registry.example.com/x","scheme":"HTTPS"}`)
	f.Add(`{"mode":"Proxy","images_repo":"registry.example.com/x","ttl":"1h"}`)
	f.Add(`{"mode":"Unmanaged","images_repo":"registry.example.com/x","user_name":"u","password":"p"}`)
	f.Add(`{"mode":"Local"}`)
	f.Add(`{}`)
	f.Add(`{"ca":"not pem"}`)
	f.Add(`{"ca":"-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----\n"}`)
	f.Add(`{"ca":"-----BEGIN CERTIFICATE-----"}`)
	f.Add(`{"generation":-1}`)
	f.Add(`{"generation":9223372036854775807}`)
	f.Add(`{"mode":"$(id)"}`)
	f.Add(`{"images_repo":"../../../etc/cron.d/x"}`)
	f.Add(`{"images_repo":"registry.example.com/x","scheme":"$(id)"}`)
	f.Add(`{"check_mode":"unknown"}`)
	f.Add(`{"MODE":"Direct"}`)
	f.Add(`{"password":"\u0000"}`)
	f.Add(`{`)
	f.Add(``)
	f.Add(`null`)

	f.Fuzz(func(t *testing.T, document string) {
		if len(document) > 1<<16 {
			return
		}

		var state ParamsState
		if err := json.Unmarshal([]byte(document), &state); err != nil {
			return
		}

		params, err := state.toParams()
		if err != nil {
			// A state that cannot be read back is reported, which is correct:
			// the CA is the only field that can fail, and an unreadable CA must
			// not silently become no CA at all.
			return
		}

		// Validate() decides; it must decide the same way twice.
		first, second := params.Validate(), params.Validate()
		if (first == nil) != (second == nil) {
			t.Fatalf("Validate() is not deterministic for %q: %v then %v", document, first, second)
		}

		// The round trip has to be lossless. Everything but the CA is copied
		// verbatim; the CA is re-encoded, so it is compared as a certificate.
		again := params.toState()

		if again.Generation != state.Generation ||
			again.Mode != state.Mode ||
			again.ImagesRepo != state.ImagesRepo ||
			again.UserName != state.UserName ||
			again.Password != state.Password ||
			again.TTL != state.TTL ||
			again.Scheme != state.Scheme ||
			again.CheckMode != state.CheckMode {
			t.Fatalf("toParams/toState changed a field of %q:\n\tbefore %+v\n\tafter  %+v\n"+
				"the orchestrator compares persisted state with computed parameters to decide "+
				"whether a registry transition is needed, so a field that changes in transit is "+
				"either a transition that never fires or one that never stops",
				document, state, again)
		}

		assertCARoundTrip(t, document, state.CA, params.CA, again.CA)
	})
}

// assertCARoundTrip checks the one field that is not copied but re-encoded.
func assertCARoundTrip(t *testing.T, document, before string, decoded *x509.Certificate, after string) {
	t.Helper()

	if before == "" {
		if decoded != nil {
			t.Fatalf("an empty CA in %q decoded to a certificate", document)
		}
		if after != "" {
			t.Fatalf("an empty CA in %q was re-encoded as %q", document, after)
		}
		return
	}

	if decoded == nil {
		t.Fatalf("a non-empty CA in %q decoded to no certificate and no error; "+
			"the upstream registry would then be trusted without the CA that was configured",
			document)
	}

	// Re-encoding must produce a certificate that decodes to the same bytes: the
	// state is what the next reconciliation compares against.
	reDecoded, err := registry_pki.DecodeCertificate([]byte(after))
	if err != nil {
		t.Fatalf("the re-encoded CA of %q does not decode: %v", document, err)
	}
	if !reDecoded.Equal(decoded) {
		t.Fatalf("the CA of %q is a different certificate after the round trip", document)
	}
}

// FuzzParamsValidate covers the parameter set the orchestrator validates before
// it acts on it.
func FuzzParamsValidate(f *testing.F) {
	modes := []string{"Direct", "Proxy", "Local", "Unmanaged", "", "unknown", "$(id)"}
	repos := []string{
		"registry.example.com/deckhouse/ee",
		"",
		"../../../etc/cron.d/x",
		"registry.example.com; return 200",
		"registry.example.com:99999/x",
	}

	for _, mode := range modes {
		for _, repo := range repos {
			f.Add(mode, repo, "HTTPS", "", "")
		}
	}
	f.Add("Direct", "registry.example.com/x", "HTTP", "user", "")
	f.Add("Direct", "registry.example.com/x", "HTTP", "", "password")
	f.Add("Direct", "registry.example.com/x", "ftp", "", "")

	f.Fuzz(func(t *testing.T, mode, imagesRepo, scheme, username, password string) {
		if len(imagesRepo) > 4096 || len(scheme) > 64 {
			return
		}

		params := Params{
			Mode:       registry_const.ModeType(mode),
			ImagesRepo: imagesRepo,
			Scheme:     scheme,
			UserName:   username,
			Password:   password,
		}

		if err := params.Validate(); err != nil {
			return
		}

		// An accepted mode has to be one the orchestrator can act on: every
		// consumer switches on it, and an unrecognised value would fall through
		// to a default branch rather than be refused here.
		switch params.Mode {
		case registry_const.ModeDirect, registry_const.ModeProxy,
			registry_const.ModeLocal, registry_const.ModeUnmanaged:
		default:
			t.Fatalf("Validate() accepted the unknown mode %q", params.Mode)
		}

		// The upstream fields are only checked in the modes that have an
		// upstream, and only there do they reach a sink.
		//
		// Local mode returns early from Validate() and so does Unmanaged with an
		// empty imagesRepo. That is sound rather than an omission: in Local mode
		// bashible.ConfigBuilder reads only the CA and the credentials from the
		// parameters, and imagesBase is the in-cluster constant, so ImagesRepo
		// and Scheme are never read. Asserting anything about them here would be
		// asserting a rule the module does not have.
		if params.Mode == registry_const.ModeLocal {
			return
		}
		if params.Mode == registry_const.ModeUnmanaged && params.ImagesRepo == "" {
			return
		}

		// Credentials travel as a pair.
		if (username == "") != (password == "") {
			t.Fatalf("Validate() accepted username %q with password %q for mode %q; "+
				"the two must be present or absent together", username, password, params.Mode)
		}

		// The scheme selects TLS handling in every generated configuration.
		if params.Scheme != "" && params.Scheme != "HTTP" && params.Scheme != "HTTPS" {
			t.Fatalf("Validate() accepted scheme %q for mode %q", params.Scheme, params.Mode)
		}
	})
}
