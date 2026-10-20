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
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

const (
	installShared      = "candi/bashible/nodelet_install.sh.tpl"
	installStep        = "candi/bashible/common-steps/all/003_install_nodelet.sh.tpl"
	prerequisitesPhase = "candi/bashible/bootstrap/01-bootstrap-prerequisites.sh.tpl"
	bootstrapPhase     = "candi/bashible/bootstrap/02-bootstrap-bashible.sh.tpl"
	nodeletDigest      = "sha256:nodelet"
)

func TestARegularRunInstallsFromTheGroupTemplate(t *testing.T) {
	chdirToRepositoryRoot(t)

	for _, phase := range []string{installStep, bootstrapPhase} {
		body := renderFile(t, phase, bashibleContext("Normal", map[string]any{"nodelet": nodeletDigest}))

		require.Contains(t, body, `rpp-get install -force "nodelet:`+nodeletDigest+`"`, phase)
		require.Contains(t, body, `nodelet first-document -node-group "worker"`, phase)
		require.Contains(t, body, `-api-servers "10.0.0.1:6443"`, phase)
		require.Contains(t, body, "systemctl enable --now nodelet.service", phase)
		require.NotContains(t, body, "--arg seed", phase)
		requireBashSyntax(t, body)
	}
}

func TestAReleaseWithoutTheAgentPackageFailsToRender(t *testing.T) {
	chdirToRepositoryRoot(t)

	for _, tc := range []struct{ template, runType string }{
		{template: installStep, runType: "Normal"},
		{template: installStep, runType: "ClusterBootstrap"},
		{template: bootstrapPhase, runType: "Normal"},
		{template: prerequisitesPhase, runType: "ClusterBootstrap"},
	} {
		content, err := os.ReadFile(tc.template)
		require.NoError(t, err)

		_, err = template.RenderTemplate(filepath.Base(tc.template), content, bashibleContext(tc.runType, map[string]any{}))

		require.ErrorContains(t, err, "images.registrypackages.nodelet", "%s, runType %s", tc.template, tc.runType)
	}
}

func TestTheStaticPodDocsKnowNoNodeWithoutTheAgent(t *testing.T) {
	chdirToRepositoryRoot(t)

	for _, path := range []string{
		"modules/040-node-manager/crds/nodestaticpodrequest.yaml",
		"modules/040-node-manager/crds/doc-ru-nodestaticpodrequest.yaml",
	} {
		doc := strings.ToLower(readFile(t, path))
		for _, claim := range []string{"edition", "\u0440\u0435\u0434\u0430\u043a\u0446\u0438", "no node agent", "\u043d\u0435 \u0440\u0430\u0431\u043e\u0442\u0430\u0435\u0442 \u0430\u0433\u0435\u043d\u0442"} {
			require.Falsef(t, strings.Contains(doc, claim), "%s says %q", path, claim)
		}
	}
}

func TestTheStepRunsWithTheLightPreparation(t *testing.T) {
	chdirToRepositoryRoot(t)

	body := renderFile(t, installStep, bashibleContext("Normal", map[string]any{"nodelet": nodeletDigest}))
	head := strings.Split(body, "\n")
	require.Contains(t, head[:min(25, len(head))], "# bashible: parallel-group=light-prep")
}

func TestANewNodeGetsTheAgentBeforeItsBundle(t *testing.T) {
	chdirToRepositoryRoot(t)

	body := renderFile(t, bootstrapPhase, bashibleContext("Normal", map[string]any{"nodelet": nodeletDigest}))

	install := strings.Index(body, "systemctl enable --now nodelet.service")
	bundle := strings.Index(body, `$(get_bundle bashible`)
	require.NotEqual(t, -1, install)
	require.NotEqual(t, -1, bundle)
	require.Less(t, install, bundle)
}

func TestTheInstallationNeedsNothingOnlyBashibleProvides(t *testing.T) {
	chdirToRepositoryRoot(t)

	shared := readFile(t, installShared)
	for _, helper := range []string{"bb-log-", "bb-sync-file", "bb-event-", "bb-flag", "bb-package-install"} {
		require.NotContains(t, shared, helper)
	}
}

func TestBashibleApiserverShipsTheInstallation(t *testing.T) {
	chdirToRepositoryRoot(t)

	werf := readFile(t, "modules/040-node-manager/images/bashible-apiserver/werf.inc.yaml")
	require.Contains(t, werf, "\n  - nodelet_install.sh.tpl\n")
}

const agentStubs = `
rpp-get() {
  echo "rpp-get $*"
  case "${!#}" in nodelet:*) cp -f "$RELEASE_NODELET" "$NODELET"; touch "$UNIT" ;; esac
}
systemctl() { echo "systemctl $*"; test -f "$UNIT"; }
`

const (
	workingAgent = "#!/bin/sh\necho \"$*\" >&2\necho 'kind: NodeConfig'\n"
	failingAgent = "#!/bin/sh\necho \"$*\" >&2\necho 'kind: Node'\nexit 1\n"
)

func runWithAgentStubs(t *testing.T, dir, script string, env ...string) (string, error) {
	t.Helper()
	bin := filepath.Join(dir, "nodelet")
	unit := filepath.Join(dir, "nodelet.service")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "release-nodelet"), []byte(workingAgent), 0o755))

	cmd := exec.Command("bash", "-c", strings.NewReplacer(
		"/opt/deckhouse/bin/nodelet", bin,
		"/etc/systemd/system/nodelet.service", unit,
		"/var/lib/nodelet", filepath.Join(dir, "lib"),
	).Replace(script))
	cmd.Env = append(os.Environ(),
		"NODELET="+bin,
		"UNIT="+unit,
		"RELEASE_NODELET="+filepath.Join(dir, "release-nodelet"),
	)
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestTheInstallationRestoresOnlyWhatIsMissing(t *testing.T) {
	chdirToRepositoryRoot(t)
	body := renderFile(t, installShared, bashibleContext("Normal", map[string]any{"nodelet": nodeletDigest}))
	forced := "rpp-get install -force nodelet:" + nodeletDigest

	for _, tc := range []struct {
		name            string
		agent           string
		unit, document  bool
		recorded        string
		env             []string
		install, writes string
		fails           bool
	}{
		{name: "a clean node", install: forced, writes: "-attempts 0"},
		{name: "the binary removed", unit: true, document: true, recorded: nodeletDigest, install: forced},
		{name: "the unit removed", agent: workingAgent, document: true, recorded: nodeletDigest, install: forced},
		{name: "an older release's agent", agent: workingAgent, unit: true, document: true, recorded: "sha256:older", install: forced},
		{name: "the document removed", agent: workingAgent, unit: true, recorded: nodeletDigest, env: []string{"MAX_RETRIES=5"}, writes: "-attempts 5"},
		{name: "the first run", agent: workingAgent, unit: true, recorded: nodeletDigest, env: []string{"MAX_RETRIES=5", "FIRST_BASHIBLE_RUN=yes"}, writes: "-attempts 0"},
		{name: "everything in place", agent: failingAgent, unit: true, document: true, recorded: nodeletDigest},
		{name: "no template", agent: failingAgent, unit: true, recorded: nodeletDigest, fails: true},
	} {
		dir := t.TempDir()
		store := filepath.Join(dir, "store")
		if tc.recorded != "" {
			require.NoError(t, os.MkdirAll(filepath.Join(store, "nodelet"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(store, "nodelet", "digest"), []byte(tc.recorded+"\n"), 0o644))
		}
		if tc.agent != "" {
			require.NoError(t, os.WriteFile(filepath.Join(dir, "nodelet"), []byte(tc.agent), 0o755))
		}
		if tc.unit {
			require.NoError(t, os.WriteFile(filepath.Join(dir, "nodelet.service"), nil, 0o644))
		}
		if tc.document {
			require.NoError(t, os.MkdirAll(filepath.Join(dir, "lib"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "lib", "nodeconfig.yaml"), []byte("kind: NodeConfig\n"), 0o600))
		}

		out, err := runWithAgentStubs(t, dir, "set -Eeuo pipefail\n"+agentStubs+body, append(tc.env, "BB_RP_INSTALLED_PACKAGES_STORE="+store)...)

		document, readErr := os.ReadFile(filepath.Join(dir, "lib", "nodeconfig.yaml"))
		if tc.fails {
			require.Error(t, err, "%s: %s", tc.name, out)
			require.NotContains(t, out, "systemctl", tc.name)
			require.Error(t, readErr, "%s: a failed first-document left a document", tc.name)
			continue
		}
		require.NoError(t, err, "%s: %s", tc.name, out)
		require.Contains(t, out, "systemctl enable --now nodelet.service", tc.name)
		require.Equal(t, "kind: NodeConfig\n", string(document), tc.name)
		if tc.install == "" {
			require.NotContains(t, out, "rpp-get", tc.name)
			require.NotContains(t, out, "systemctl try-restart", tc.name)
		} else {
			require.Contains(t, out, tc.install, tc.name)
			require.Contains(t, out, "systemctl try-restart nodelet.service", tc.name)
		}
		if tc.writes == "" {
			require.NotContains(t, out, "first-document", tc.name)
		} else {
			require.Contains(t, out, tc.writes, tc.name)
		}
		require.Equal(t, tc.install != "", timedSection(out, "package"), tc.name)
		require.Equal(t, tc.writes != "", timedSection(out, "document"), tc.name)
		require.True(t, timedSection(out, "start"), tc.name)
	}
}

func timedSection(out, section string) bool {
	return regexp.MustCompile(`(?m)^\[bashible-timing\] step=nodelet_install section=` + section + ` dur=[0-9]+\.[0-9]{3}s$`).MatchString(out)
}

func TestAStaticFirstMasterGetsTheAgentAfterItsAddress(t *testing.T) {
	chdirToRepositoryRoot(t)

	for _, tc := range []struct {
		name, ip string
		installs bool
	}{
		{name: "an address in internalNetworkCIDRs", ip: `echo "2: eth0    inet 10.0.0.5/24 brd 10.0.0.255 scope global eth0"`, installs: true},
		{name: "ip failing", ip: "return 1"},
	} {
		out, discovered, err := runStaticFirstMasterPrerequisites(t, tc.ip)

		if !tc.installs {
			require.Error(t, err, "%s: %s", tc.name, out)
			require.NotContains(t, out, "rpp-get install -force nodelet:", tc.name)
			continue
		}
		require.NoError(t, err, "%s: %s", tc.name, out)
		require.Equal(t, "10.0.0.5\n", discovered, tc.name)
		require.Contains(t, out, "rpp-get install -force nodelet:"+nodeletDigest, tc.name)
		require.Contains(t, out, "systemctl enable --now nodelet.service", tc.name)
	}
}

func runStaticFirstMasterPrerequisites(t *testing.T, ip string) (string, string, error) {
	t.Helper()
	dir := t.TempDir()

	context := bashibleContext("ClusterBootstrap", map[string]any{"nodelet": nodeletDigest})
	context["nodeGroup"] = map[string]any{
		"name":     "master",
		"nodeType": "Static",
		"static":   map[string]any{"internalNetworkCIDRs": []any{"10.0.0.0/24"}},
	}
	body := strings.NewReplacer(
		"/opt/deckhouse/bin/rpp-get", "rpp-get",
		"/var/lib/bashible", filepath.Join(dir, "bashible"),
		"/etc/os-release", filepath.Join(dir, "os-release"),
	).Replace(renderFile(t, prerequisitesPhase, context))

	const firstCommand = "export PACKAGES_PROXY_BOOTSTRAP_CLUSTER_UUID="
	require.Contains(t, body, firstCommand)
	stubs := `
bb-minget-install() { :; }
bb-rpp-get-install() { :; }
bb-discover-node-name() { :; }
bb-d8-node-name() { echo master-0; }
ip() { ` + ip + `; }
jq() { echo 'kind: NodeConfig'; }
` + agentStubs

	out, err := runWithAgentStubs(t, dir, strings.Replace(body, firstCommand, stubs+firstCommand, 1))
	discovered, readErr := os.ReadFile(filepath.Join(dir, "bashible", "discovered-node-ip"))
	require.NoError(t, readErr, "%s", out)
	return out, string(discovered), err
}

func renderFile(t *testing.T, path string, context map[string]any) string {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	rendered, err := template.RenderTemplate(filepath.Base(path), content, context)
	require.NoError(t, err, "render %s", path)
	return rendered.Content.String()
}

func bashibleContext(runType string, packages map[string]any) map[string]any {
	return map[string]any{
		"runType":                       runType,
		"nodeGroup":                     map[string]any{"name": "worker", "nodeType": "CloudEphemeral"},
		"clusterMasterKubeAPIEndpoints": []any{"10.0.0.1:6443"},
		"images":                        map[string]any{"registrypackages": packages},
		"nodeletSeed":                   map[string]any{"kind": "NodeConfig", "spec": map[string]any{"systemType": "Mutable"}},
	}
}
