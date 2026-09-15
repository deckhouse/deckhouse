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

// Fuzz harnesses for the producer of the bashible registry configuration.
//
// Threat model coverage (registry-threat-model.md), harness 10 and the input
// side of harness 2. This is the code site section 6 names first: the hook that
// turns cluster state into the d8-system/registry-bashible-config secret every
// node reads. Two of its inputs come from objects the module does not own:
//
//   - MasterNodesIPs is collected in k8s.go from
//     Node.status.addresses[type=InternalIP], filtered only for being non-empty,
//     and reaches `server <value>;` in the node load balancer's NGINX
//     configuration through registry_const.GenerateProxyEndpoints.
//   - The Unmanaged mode parameters are read from the deckhouse-registry secret,
//     whose ImagesRepo becomes the imagesBase every node pulls from and a mirror
//     host interpolated into /etc/containerd/registry.d.
//
// TM-17 and TM-18 are both assessed as critical, and both are about exactly
// this: a value from cluster state reaching those two sinks.
//
// The oracle is not "the result validates". Bounding these values belongs to the
// consumer, and bashible-apiserver does it when it reads the secret; the rules
// themselves are pinned by the tests next to the model, in
// go_lib/registry/models/bashible. What this harness holds build() to is what
// only build() can get wrong: reporting the addresses it was given, in order,
// and reporting them the same way twice. A fabricated, dropped or reordered
// endpoint validates perfectly well and points a node at the wrong master.

package bashible

import (
	"testing"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

func FuzzConfigBuilderMasterNodesIPs(f *testing.F) {
	// The master addresses as k8s.go collects them, then the values a Node
	// object could carry instead. The bool picks Local over Proxy: both modes
	// produce endpoints, and the placeholder path differs between them.
	f.Add("10.0.0.1", false)
	f.Add("10.0.0.1", true)
	f.Add("10.0.0.1,10.0.0.2,10.0.0.3", false)
	f.Add("10.0.0.1,10.0.0.2,10.0.0.3", true)
	f.Add("", false)
	f.Add(",", false)
	f.Add(",,", true)
	f.Add("10.0.0.1,", false)
	f.Add("fd00::1", false)
	f.Add("[fd00::1]", true)
	f.Add("10.0.0.1:5001", false)
	f.Add("10.0.0.1; return 200", false)
	f.Add("10.0.0.1;}\nserver { listen 127.0.0.1:5002; proxy_pass evil; }", false)
	f.Add("10.0.0.1 # comment", true)
	f.Add("$(id > /tmp/pwned)", false)
	f.Add("`id`", true)
	f.Add("${IFS}", false)
	f.Add("../../../etc/cron.d/x", false)
	f.Add("999.999.999.999", true)
	f.Add("\x00", false)

	f.Fuzz(func(t *testing.T, joined string, useLocal bool) {
		if len(joined) > 4096 {
			return
		}

		params := ProxyLocalModeParams{
			Username: "ro-user",
			Password: "ro-password",
		}

		builder := ConfigBuilder{MasterNodesIPs: splitFuzzList(joined)}
		if useLocal {
			builder.ModeParams = ModeParams{Local: &params}
		} else {
			builder.ModeParams = ModeParams{Proxy: &params}
		}

		config, err := builder.build()
		if err != nil {
			// A refusal to build is a correct outcome for a hostile input.
			return
		}

		assertProducerInvariants(t, builder, config, "MasterNodesIPs", builder.MasterNodesIPs)
	})
}

// FuzzConfigBuilderUnmanaged covers the Unmanaged parameters, which the hook
// reads from the deckhouse-registry secret rather than from the ModuleConfig,
// so they do not pass the ModuleConfig validation on the way in.
func FuzzConfigBuilderUnmanaged(f *testing.F) {
	// The Unmanaged parameters as fromRegistrySecret builds them from the
	// deckhouse-registry secret: the address and the scheme, neither validated
	// on the way in. The scheme is lowercased there, so its casing is varied too.
	f.Add("registry.example.com/deckhouse/ee", "https")
	f.Add("registry.example.com:5000/deckhouse/ee", "https")
	f.Add("registry.example.com/deckhouse/ee", "http")
	f.Add("registry.example.com/deckhouse/ee", "HTTPS")
	f.Add("registry.example.com/deckhouse/ee", "")
	f.Add("registry.example.com/deckhouse/ee", "ftp")
	f.Add("registry.example.com", "https")
	f.Add("registry.example.com/", "https")
	f.Add("", "https")
	f.Add("/", "https")
	f.Add("registry.example.com/a/../../b", "https")
	f.Add("../../../etc/cron.d/x", "https")
	f.Add("registry.example.com; return 200", "https")
	f.Add("registry.example.com$(id)", "https")
	f.Add("`id`/x", "https")
	f.Add("registry.example.com\nserver 127.0.0.1:5002", "https")
	f.Add("[fd00::1]:5000/x", "https")
	f.Add("registry.example.com:99999/x", "https")
	f.Add("registry.example.com/deckhouse/ee:tag", "https")
	f.Add("registry.example.com/deckhouse/ee", "https\nskip_verify = true")

	f.Fuzz(func(t *testing.T, imagesRepo, scheme string) {
		if len(imagesRepo) > 4096 || len(scheme) > 64 {
			return
		}

		builder := ConfigBuilder{
			ModeParams: ModeParams{Unmanaged: &UnmanagedModeParams{
				ImagesRepo: imagesRepo,
				Scheme:     scheme,
				Username:   "ro-user",
				Password:   "ro-password",
			}},
		}

		config, err := builder.build()
		if err != nil {
			return
		}

		assertProducerInvariants(t, builder, config, "Unmanaged.ImagesRepo", []string{imagesRepo, scheme})
	})
}

// assertProducerInvariants states what build() itself has to guarantee.
//
// Not that the result validates: bounding these values is the consumer's job,
// and bashible-apiserver does it when it reads the secret. A check here would be
// a check on Deckhouse's own internal state, where a false positive fails the
// orchestrator hook rather than one field -- see the note in build().
//
// What build() does owe its caller is that it reports the addresses it was given
// and nothing else. Fabricating, dropping or reordering an endpoint would
// misattribute it to another node, and that is invisible to the consumer: every
// value would still validate, and the wrong node would be proxied to.
func assertProducerInvariants(t *testing.T, builder ConfigBuilder, config *Config, field string, values []string) {
	t.Helper()

	if config == nil {
		t.Fatalf("build() returned no config and no error for %s = %q", field, values)
	}

	want := registry_const.GenerateProxyEndpoints(builder.MasterNodesIPs)
	if len(builder.MasterNodesIPs) == 0 {
		want = []string{}
	}

	// Only Proxy and Local produce endpoints at all; the other modes produce an
	// empty list regardless of the addresses collected.
	produces := builder.ModeParams.Proxy != nil || builder.ModeParams.Local != nil
	if !produces {
		if len(config.ProxyEndpoints) != 0 {
			t.Fatalf("build() produced %d endpoint(s) in a mode that has no balancer, from %s = %q",
				len(config.ProxyEndpoints), field, values)
		}
		return
	}

	if len(config.ProxyEndpoints) != len(want) {
		t.Fatalf("build() produced %d endpoint(s) for %d address(es), from %s = %q:\n\tgot  %q\n\twant %q",
			len(config.ProxyEndpoints), len(want), field, values, config.ProxyEndpoints, want)
	}
	for i := range want {
		if config.ProxyEndpoints[i] != want[i] {
			t.Fatalf("endpoint[%d] is %q, expected %q, from %s = %q; the order has to hold or an "+
				"endpoint stops being attributable to the node it came from",
				i, config.ProxyEndpoints[i], want[i], field, values)
		}
	}

	// The version is what every node compares against to decide whether it is up
	// to date, so an empty one would stall the rollout silently.
	if config.Version == "" {
		t.Fatalf("build() produced no version, from %s = %q", field, values)
	}

	// Building twice from the same inputs has to give the same version, or the
	// nodes chase a version that keeps changing.
	again, err := builder.build()
	if err != nil {
		t.Fatalf("build() succeeded and then failed for the same inputs: %v", err)
	}
	if again.Version != config.Version {
		t.Fatalf("build() is not deterministic for %s = %q: %q then %q",
			field, values, config.Version, again.Version)
	}
}

// splitFuzzList reads the fuzzed string as a comma-separated list, so one string
// argument still varies the number of master nodes.
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
