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

// Fuzz harness for the transfer configuration of registry-syncer.
//
// Threat model coverage (registry-threat-model.md), harness 7. The document
// keeps this one outside the threat perimeter and marks it optional: TM-16 was
// assessed as not applicable, because the utility runs only during initial
// cluster bootstrap in Local mode, where the deploying subject already has full
// access to the environment it is creating. So this is a robustness test, not a
// security control -- what it protects is the bootstrap finishing, since a
// configuration the syncer accepts and then cannot act on stops the cluster
// coming up.
//
// The invariants are the same two that matter for the mirrorer: an accepted
// address must be one the registry client can construct, and accepted
// credentials must be carriable in an HTTP header.

package config

import (
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
)

func FuzzSyncerConfig(f *testing.F) {
	// The transfer configuration as the bootstrap writes it, then the addresses
	// the registry client cannot construct and the credentials that cannot
	// travel in a header.
	f.Add(`{"source":{"address":"registry.example.com:5000"},"destination":{"address":"127.0.0.1:5001"}}`)
	f.Add("source:\n  address: registry.example.com:5000\n  user:\n    name: u\n    password: p\ndestination:\n  address: 127.0.0.1:5001\n")
	f.Add(`{}`)
	f.Add(``)
	f.Add(`null`)
	f.Add(`[]`)
	f.Add(`{`)
	f.Add(`{"source":{},"destination":{}}`)
	f.Add(`{"source":{"address":""},"destination":{"address":""}}`)
	f.Add(`{"source":{"address":"registry.example.com"}}`)
	f.Add(`{"source":{"address":"https://registry.example.com"},"destination":{"address":"127.0.0.1:5001"}}`)
	f.Add(`{"source":{"address":"registry.example.com/path"},"destination":{"address":"127.0.0.1:5001"}}`)
	f.Add(`{"source":{"address":"registry.example.com:99999"},"destination":{"address":"127.0.0.1:5001"}}`)
	f.Add(`{"source":{"address":"[fd00::1]:5000"},"destination":{"address":"127.0.0.1:5001"}}`)
	f.Add(`{"source":{"address":"$(id)"},"destination":{"address":"127.0.0.1:5001"}}`)
	f.Add(`{"source":{"address":"registry.example.com","ca":"-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----\n"},"destination":{"address":"127.0.0.1:5001"}}`)
	f.Add(`{"source":{"address":"registry.example.com","user":{"name":"u\r\nX-Evil: 1","password":"p"}},"destination":{"address":"127.0.0.1:5001"}}`)
	f.Add(`{"source":{"address":"registry.example.com","user":{"name":"u","password":"p\nq"}},"destination":{"address":"127.0.0.1:5001"}}`)
	f.Add(`{"source":{"address":"registry.example.com","user":{"name":"u","password":""}},"destination":{"address":"127.0.0.1:5001"}}`)
	f.Add("source: [1,2]\n")

	f.Fuzz(func(t *testing.T, document string) {
		// YAML parsing is superlinear in the number of nodes, and a document
		// larger than this is not a configuration this file could ever be.
		if len(document) > 8<<10 {
			return
		}

		config, err := FromBytes([]byte(document))
		if err != nil {
			return
		}

		// Parsing has to be a function of the input: the syncer reads this file
		// once per bootstrap attempt, and a retry must see the same thing.
		again, againErr := FromBytes([]byte(document))
		if againErr != nil {
			t.Fatalf("FromBytes succeeded and then failed for the same document: %v", againErr)
		}
		if again.Src.Address != config.Src.Address || again.Dest.Address != config.Dest.Address {
			t.Fatalf("FromBytes is not deterministic for %q", document)
		}

		if err := config.Validate(); err != nil {
			return
		}

		for role, registry := range map[string]Registry{
			"source":      config.Src,
			"destination": config.Dest,
		} {
			if _, err := name.NewRegistry(registry.Address); err != nil {
				t.Fatalf("Validate() accepted the %s address %q, which name.NewRegistry rejects: "+
					"%v; the syncer would pass its own validation and then fail to connect, "+
					"stopping the initial cluster bootstrap", role, registry.Address, err)
			}

			if registry.User == nil {
				continue
			}
			assertHeaderSafe(t, role+".user.name", registry.User.Name)
			assertHeaderSafe(t, role+".user.password", registry.User.Password)
		}
	})
}

// assertHeaderSafe fails if a credential could not be carried in an HTTP header.
func assertHeaderSafe(t *testing.T, field, value string) {
	t.Helper()

	if i := strings.IndexAny(value, "\r\n\x00"); i >= 0 {
		t.Fatalf("Validate() accepted %s = %q, which carries %q at offset %d; it is sent in the "+
			"Authorization header of every request to the registry", field, value, value[i], i)
	}
}
