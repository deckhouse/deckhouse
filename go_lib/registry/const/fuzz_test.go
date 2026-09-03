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

// Fuzz harness for the producer of `proxyEndpoints`.
//
// Threat model coverage (registry-threat-model.md), harness 10: this is one of
// the three code sites section 6 names for that value, and the one that turns a
// node address into the string a node's NGINX will contain. The other two are
// hooks/orchestrator/bashible (see its own harness) and
// go_lib/registry/models/bashible, which validates the result.
//
// GenerateProxyEndpoints has no validation of its own and should not acquire
// any: its job is to join a host and a port. What it must never do is *launder*
// its input -- produce, from a value that would be rejected, something that
// passes validation. The oracle below states exactly that, in both directions,
// so the pairing between this producer and helpers.ProxyEndpoint stays honest.

package constant

import (
	"net"
	"strconv"
	"testing"

	"github.com/deckhouse/deckhouse/go_lib/registry/helpers"
)

func FuzzGenerateProxyEndpoints(f *testing.F) {
	f.Add("10.0.0.1")
	f.Add("192.168.1.1")
	f.Add("")
	f.Add("fd00::1")
	f.Add("::1")
	f.Add("[fd00::1]")
	f.Add("0.0.0.0")
	f.Add("255.255.255.255")
	f.Add("10.0.0.1:5001")
	f.Add("localhost")
	f.Add("registry.example.com")
	// The shapes that must not survive into something acceptable.
	f.Add("10.0.0.1; return 200")
	f.Add("10.0.0.1$(id)")
	f.Add("`id`")
	f.Add("10.0.0.1\nserver 127.0.0.1:5002")
	f.Add("${discovered_node_ip}")
	f.Add("10.0.0.1 10.0.0.2")
	f.Add(" 10.0.0.1 ")
	f.Add("\x00")
	f.Add("999.999.999.999")

	f.Fuzz(func(t *testing.T, address string) {
		if len(address) > 512 {
			return
		}

		endpoints := GenerateProxyEndpoints([]string{address})

		// One input, one output: the producer neither drops nor invents entries.
		if len(endpoints) != 1 {
			t.Fatalf("GenerateProxyEndpoints(%q) returned %d endpoints, expected 1",
				address, len(endpoints))
		}

		// The result must be exactly the host and the module's port, joined the
		// way net.JoinHostPort joins them. Concatenating instead would drop the
		// brackets an IPv6 literal needs, which is how the port ends up read as
		// part of the address.
		want := net.JoinHostPort(address, strconv.Itoa(Port))
		if endpoints[0] != want {
			t.Fatalf("GenerateProxyEndpoints(%q) produced %q, expected %q",
				address, endpoints[0], want)
		}

		accepted := helpers.ProxyEndpoint(endpoints[0]) == nil

		// A literal IP address is the input this producer exists for, so its
		// output has to be something the validator downstream accepts.
		if net.ParseIP(address) != nil && !accepted {
			t.Fatalf("GenerateProxyEndpoints(%q) produced %q, which helpers.ProxyEndpoint "+
				"rejects, yet %q is a valid IP address; the producer and the validator "+
				"must agree on the shape they exchange", address, endpoints[0], address)
		}

		// The other direction is the one that matters for TM-17. Anything that is
		// not an address must not come out looking like one: this producer is
		// upstream of validation, and a value it laundered would reach
		// `server <value>;` with nothing left to catch it.
		if net.ParseIP(address) == nil && accepted && address != helpers.NodeIPPlaceholder {
			t.Fatalf("GenerateProxyEndpoints(%q) produced %q, which passes "+
				"helpers.ProxyEndpoint even though the input is not an IP address; "+
				"the producer must not turn an unacceptable value into an acceptable one",
				address, endpoints[0])
		}
	})
}

// FuzzGenerateProxyEndpointsList covers the list behaviour: order and count are
// what tie an endpoint back to the node it came from.
func FuzzGenerateProxyEndpointsList(f *testing.F) {
	f.Add("10.0.0.1,10.0.0.2,10.0.0.3")
	f.Add("")
	f.Add(",")
	f.Add("10.0.0.1,10.0.0.1")
	f.Add("fd00::1,10.0.0.1")
	f.Add("10.0.0.1,; return 200,10.0.0.2")

	f.Fuzz(func(t *testing.T, joined string) {
		if len(joined) > 4096 {
			return
		}

		addresses := splitFuzzList(joined)
		endpoints := GenerateProxyEndpoints(addresses)

		if len(endpoints) != len(addresses) {
			t.Fatalf("GenerateProxyEndpoints(%q) returned %d endpoints for %d addresses",
				addresses, len(endpoints), len(addresses))
		}

		for i, address := range addresses {
			want := net.JoinHostPort(address, strconv.Itoa(Port))
			if endpoints[i] != want {
				t.Fatalf("endpoint[%d] is %q for address %q, expected %q; the order must be "+
					"preserved so that an endpoint stays attributable to its node",
					i, endpoints[i], address, want)
			}
		}
	})
}

// splitFuzzList reads the fuzzed string as a comma-separated list, which keeps
// the target's signature to one string while still varying the list length.
func splitFuzzList(joined string) []string {
	if joined == "" {
		return nil
	}

	var (
		items   []string
		current []byte
	)
	for i := 0; i < len(joined); i++ {
		if joined[i] == ',' {
			items = append(items, string(current))
			current = current[:0]
			continue
		}
		current = append(current, joined[i])
	}
	return append(items, string(current))
}
