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

// Fuzz harness for the mirroring configuration.
//
// Threat model coverage (registry-threat-model.md), harness 6: correct parsing
// of the Upstreams list, the accounts and the replica addresses, and behaviour
// when a replica is unreachable or an address is malformed. This is the
// consuming side -- the file the mirrorer reads at startup, written for it by
// nodeservices-manager from the template in internal/staticpod. The producing
// side has its own harness there (FuzzMirrorerConfigUpstreams); this one is
// about what happens when the file that arrives is not the file that was meant.
//
// Two invariants carry the value:
//
//   - Anything Validate() accepts must be constructible. The addresses go to
//     name.NewRegistry and the credentials into the Authorization header of
//     every request to a replica. A configuration that passes validation and
//     then fails to construct is a mirrorer that exits at startup, which stops
//     replication between the registries every node pulls from.
//   - The credentials must be usable in a header. They are the read-only and
//     push accounts of the in-cluster registry, and a line break in one of them
//     would split the request that carries it.

package config

import (
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
)

// configSeeds are the documents this file could plausibly arrive as: the shape
// the template produces, the shapes it never would, and the shapes that are not
// a configuration at all.
var configSeeds = []string{
	`{"users":{"puller":{"name":"p","password":"x"},"pusher":{"name":"q","password":"y"}},"local":"127.0.0.1:5001","remote":["10.0.0.1:5001"]}`,
	"users:\n  puller:\n    name: p\n    password: x\n  pusher:\n    name: q\n    password: y\nlocal: \"127.0.0.1:5001\"\nremote:\n  - \"10.0.0.1:5001\"\n",
	`{}`,
	``,
	`null`,
	`[]`,
	`{"local":"","remote":[]}`,
	`{"users":{},"local":"127.0.0.1:5001","remote":["10.0.0.1:5001"]}`,
	// Addresses the producer would never emit.
	`{"users":{"puller":{"name":"p","password":"x"},"pusher":{"name":"q","password":"y"}},"local":"127.0.0.1","remote":["10.0.0.1"]}`,
	`{"users":{"puller":{"name":"p","password":"x"},"pusher":{"name":"q","password":"y"}},"local":"127.0.0.1:99999","remote":["10.0.0.1:5001"]}`,
	`{"users":{"puller":{"name":"p","password":"x"},"pusher":{"name":"q","password":"y"}},"local":"http://127.0.0.1:5001","remote":["10.0.0.1:5001"]}`,
	`{"users":{"puller":{"name":"p","password":"x"},"pusher":{"name":"q","password":"y"}},"local":"127.0.0.1:5001/path","remote":["10.0.0.1:5001"]}`,
	`{"users":{"puller":{"name":"p","password":"x"},"pusher":{"name":"q","password":"y"}},"local":"[fd00::1]:5001","remote":["[fd00::2]:5001"]}`,
	`{"users":{"puller":{"name":"p","password":"x"},"pusher":{"name":"q","password":"y"}},"local":"127.0.0.1:5001","remote":["","10.0.0.1:5001"]}`,
	// Credentials that would split the request that carries them. The escapes
	// are JSON, so the parsed values hold real control characters.
	`{"users":{"puller":{"name":"p\r\nX-Evil: 1","password":"x"},"pusher":{"name":"q","password":"y"}},"local":"127.0.0.1:5001","remote":["10.0.0.1:5001"]}`,
	`{"users":{"puller":{"name":"p","password":"x\ny"},"pusher":{"name":"q","password":"y"}},"local":"127.0.0.1:5001","remote":["10.0.0.1:5001"]}`,
	`{"users":{"puller":{"name":"p","password":"x\u0000y"},"pusher":{"name":"q","password":"y"}},"local":"127.0.0.1:5001","remote":["10.0.0.1:5001"]}`,
	// Bounds.
	`{"users":{"puller":{"name":"p","password":"x"},"pusher":{"name":"q","password":"y"}},"local":"127.0.0.1:5001","remote":["10.0.0.1:5001"],"parallelizm":-1}`,
	`{"users":{"puller":{"name":"p","password":"x"},"pusher":{"name":"q","password":"y"}},"local":"127.0.0.1:5001","remote":["10.0.0.1:5001"],"sleep":-1}`,
	"local: [1,2]\n",
	"ca: /pki/ca.crt\n",
}

func FuzzParseConfig(f *testing.F) {
	for _, seed := range configSeeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, document string) {
		// YAML parsing is superlinear in the number of nodes, and a document
		// larger than this is not a configuration this file could ever be.
		if len(document) > 8<<10 {
			return
		}

		config, err := parse(strings.NewReader(document))
		if err != nil {
			return
		}

		// Parsing has to be a function of the input, not of the run: the
		// mirrorer re-reads this file when the static pod restarts.
		again, againErr := parse(strings.NewReader(document))
		if againErr != nil {
			t.Fatalf("parse succeeded and then failed for the same document: %v", againErr)
		}
		if again.LocalAddress != config.LocalAddress ||
			len(again.RemoteAddresses) != len(config.RemoteAddresses) {
			t.Fatalf("parse is not deterministic for %q", document)
		}

		if err := config.Validate(); err != nil {
			return
		}

		// Every accepted address must be one the registry client can construct;
		// otherwise validation passes and the mirrorer exits at startup instead.
		if _, err := name.NewRegistry(config.LocalAddress); err != nil {
			t.Fatalf("Validate() accepted the local address %q, which name.NewRegistry rejects: "+
				"%v; the mirrorer would pass its own validation and then fail to start, leaving "+
				"the registries every node pulls from unreplicated", config.LocalAddress, err)
		}
		for i, address := range config.RemoteAddresses {
			if _, err := name.NewRegistry(address); err != nil {
				t.Fatalf("Validate() accepted remote[%d] = %q, which name.NewRegistry rejects: %v",
					i, address, err)
			}
		}

		// The credentials go into an Authorization header on every request.
		assertHeaderSafe(t, "puller.name", config.Users.Puller.Name)
		assertHeaderSafe(t, "puller.password", config.Users.Puller.Password)
		assertHeaderSafe(t, "pusher.name", config.Users.Pusher.Name)
		assertHeaderSafe(t, "pusher.password", config.Users.Pusher.Password)

		// A negative limit is not a small limit: errgroup.SetLimit reads it as
		// no limit at all, so the mirrorer would pull from every repository at
		// once against the registry the whole cluster depends on.
		if config.Parallelizm < 0 {
			t.Fatalf("Validate() accepted parallelizm %d; a negative value removes the "+
				"concurrency limit rather than lowering it", config.Parallelizm)
		}
	})
}

// assertHeaderSafe fails if a credential could not be carried in an HTTP header.
func assertHeaderSafe(t *testing.T, field, value string) {
	t.Helper()

	if i := strings.IndexAny(value, "\r\n\x00"); i >= 0 {
		t.Fatalf("Validate() accepted %s = %q, which carries %q at offset %d; it is sent in the "+
			"Authorization header of every request to a replica", field, value, value[i], i)
	}
}
