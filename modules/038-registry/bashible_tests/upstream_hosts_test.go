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

package bashible_tests

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	gotoml "github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

// unmanagedRegistryWithUpstreams is what bashible-apiserver hands a node with no agent once
// node-controller has accepted RegistryUpstream objects: the cluster's registry from
// `deckhouse-registry` in the hosts, and the upstreams apart from them, for the one runtime
// that reads its mirrors inline (pkg/template/registry/upstreams.go there).
func unmanagedRegistryWithUpstreams() map[string]any {
	return map[string]any{
		"registryModuleEnable": false,
		"mode":                 "unmanaged",
		"version":              "unknown",
		"imagesBase":           "registry.example.com/deckhouse/ce",
		"hosts": map[string]any{
			"registry.example.com": map[string]any{
				"mirrors": []any{map[string]any{"scheme": "https", "host": "registry.example.com",
					"auth": map[string]any{"username": "", "password": "", "auth": "dXNlcm5hbWU6cGFzc3dvcmQ="}}},
			},
		},
		"inlineUpstreams": map[string]any{
			"images.example.com": map[string]any{
				"mirrors": []any{
					map[string]any{"scheme": "https", "host": "vendor.example.com", "ca": "VENDOR-CA",
						"auth": map[string]any{"username": "", "password": "", "auth": "dXNlcm5hbWU6cGFzc3dvcmQ="}},
					map[string]any{"scheme": "http", "host": "mirror.example.com:5000",
						"auth": map[string]any{"username": "", "password": "", "auth": ""}},
				},
			},
		},
	}
}

// renderOn renders a step for a node on the given runtime.
func renderOn(t *testing.T, step, cri string, registry map[string]any) string {
	t.Helper()
	path := filepath.Join(candi, step)
	content, err := os.ReadFile(path)
	require.NoError(t, err, "cannot read %s", path)
	ctx := nodeContext(t, registry)
	ctx["cri"] = cri
	rendered, err := template.RenderTemplate(filepath.Base(step), content, ctx)
	require.NoError(t, err, "cannot render %s", step)
	return rendered.Content.String()
}

// registry.d belongs to nodelet for the upstreams: the step that writes it renders the
// cluster's registry alone and records nothing else in its state file, so the two writers
// never share a directory.
func TestRegistryDLeavesTheUpstreamsToNodelet(t *testing.T) {
	rendered := effective(render(t, "all/030_configure_containerd_registry.sh.tpl", unmanagedRegistryWithUpstreams()))

	require.Contains(t, rendered, `mkdir -p "/etc/containerd/registry.d/registry.example.com"`)
	require.NotContains(t, rendered, "images.example.com")
	require.NotContains(t, rendered, "vendor.example.com")
}

// containerd v1 without the module reads inline mirrors: the upstreams are mirrors there
// like the cluster's registry, with their own credentials and authority.
func TestContainerdV1RoutesTheUpstreamsInline(t *testing.T) {
	rendered := renderOn(t, "all/032_configure_containerd.sh.tpl", "Containerd", unmanagedRegistryWithUpstreams())

	for _, want := range []string{
		`registry.mirrors."registry.example.com"]`,
		`registry.mirrors."images.example.com"]`,
		`endpoint = ["https://vendor.example.com", "http://mirror.example.com:5000"]`,
		`registry.configs."vendor.example.com".auth]`,
		`ca_file = "/opt/deckhouse/share/ca-certificates/registry-vendor.example.com-ca.crt"`,
		`registry.configs."mirror.example.com:5000".tls]`,
	} {
		require.Contains(t, rendered, want)
	}
}

// The authority of an inline upstream lands where the inline configuration points, on the
// runtime that reads it and nowhere else.
func TestOnlyContainerdV1GetsTheInlineAuthorities(t *testing.T) {
	const step = "all/003_install_ca_certificates.sh.tpl"
	const file = "/opt/deckhouse/share/ca-certificates/registry-vendor.example.com-ca.crt"

	require.Contains(t, renderOn(t, step, "Containerd", unmanagedRegistryWithUpstreams()), file)
	require.NotContains(t, renderOn(t, step, "ContainerdV2", unmanagedRegistryWithUpstreams()), file)
}

// While the module configures the nodes, containerd v1 reads registry.d (step 032 sets
// config_path), where nodelet writes each route's authority beside its hosts.toml. A context
// that carried inline upstreams anyway must not put their authorities into the system trust
// store as well.
func TestTheInlineAuthoritiesFollowTheModuleBeingDisabled(t *testing.T) {
	const step = "all/003_install_ca_certificates.sh.tpl"
	const file = "/opt/deckhouse/share/ca-certificates/registry-vendor.example.com-ca.crt"

	withModule := unmanagedRegistryWithUpstreams()
	withModule["registryModuleEnable"] = true

	require.NotContains(t, renderOn(t, step, "Containerd", withModule), file)
	require.NotContains(t, renderOn(t, "all/032_configure_containerd.sh.tpl", "Containerd", withModule), `registry.mirrors."images.example.com"]`)
}

// containerdV1Config is the config.toml step 032 writes for a containerd v1 node, with the
// shell variables of the heredoc filled in.
func containerdV1Config(t *testing.T, registry, moduleSourcesCA map[string]any) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(candi, "all/032_configure_containerd.sh.tpl"))
	require.NoError(t, err)
	ctx := nodeContext(t, registry)
	ctx["normal"] = map[string]any{"moduleSourcesCA": moduleSourcesCA}
	rendered, err := template.RenderTemplate("032_configure_containerd.sh.tpl", content, ctx)
	require.NoError(t, err)

	const opening = "bb-sync-file /etc/containerd/deckhouse.toml - << EOF\n"
	out := rendered.Content.String()
	start := strings.Index(out, opening)
	require.GreaterOrEqual(t, start, 0, "no config.toml in the step")
	body := out[start+len(opening):]
	body = body[:strings.Index(body, "\nEOF\n")]
	return regexp.MustCompile(`\$\{[a-z_]+\}`).ReplaceAllString(body, "1")
}

// config.toml keys the credentials and the authority of a registry by its host, and
// containerd v1 refuses the whole file on a table defined twice: every node of the runtime
// would then lose containerd. So each registry gets one table, whoever names it — the
// cluster, an upstream over both schemes, or a ModuleSource whose registry an upstream
// names — and the file is parsed the way containerd v1 parses it (go-toml v1).
func TestContainerdV1ConfigParsesWithTheUpstreamsInline(t *testing.T) {
	sameRegistry := unmanagedRegistryWithUpstreams()
	sameRegistry["inlineUpstreams"] = map[string]any{
		"images.example.com": map[string]any{"mirrors": []any{
			map[string]any{"scheme": "https", "host": "modules.example.com", "auth": map[string]any{"auth": "dXNlcm5hbWU6cGFzc3dvcmQ="}},
			map[string]any{"scheme": "http", "host": "Modules.Example.com"},
		}},
	}
	withoutUpstreams := unmanagedRegistryWithUpstreams()
	delete(withoutUpstreams, "inlineUpstreams")

	for name, tc := range map[string]struct {
		registry, moduleSourcesCA map[string]any
		want                      []string
	}{
		"the cluster's registry alone": {registry: withoutUpstreams, moduleSourcesCA: map[string]any{"Modules.Example.com": "SOURCE-CA"}, want: []string{
			`[plugins."io.containerd.grpc.v1.cri".registry.configs."modules.example.com".tls]`,
		}},
		"the upstreams inline": {registry: unmanagedRegistryWithUpstreams(), moduleSourcesCA: map[string]any{}, want: []string{
			`registry.configs."vendor.example.com".auth]`,
			`ca_file = "/opt/deckhouse/share/ca-certificates/registry-vendor.example.com-ca.crt"`,
		}},
		"an upstream on the registry of a ModuleSource": {registry: sameRegistry, moduleSourcesCA: map[string]any{"Modules.Example.com": "SOURCE-CA"}, want: []string{
			`ca_file = "/opt/deckhouse/share/ca-certificates/modules.example.com-ca.crt"`,
		}},
	} {
		t.Run(name, func(t *testing.T) {
			config := containerdV1Config(t, tc.registry, tc.moduleSourcesCA)
			_, err := gotoml.Load(config)
			require.NoError(t, err, "config.toml:\n%s", config)
			require.Equal(t, 1, strings.Count(config, `[plugins."io.containerd.grpc.v1.cri".registry.configs]`))
			for _, want := range tc.want {
				require.Contains(t, config, want)
			}
		})
	}
}
