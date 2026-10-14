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
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/itchyny/gojq"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/nodeletseed"
	"github.com/deckhouse/deckhouse/testing/library"
)

const (
	caStep        = "candi/bashible/common-steps/all/003_install_ca_certificates.sh.tpl"
	mandatoryStep = "candi/bashible/common-steps/all/004_install_mandatory_packages.sh.tpl"
	kubeletStep   = "candi/bashible/common-steps/all/062_install_kubelet_and_his_friends.sh.tpl"
	prefetchStep  = "candi/bashible/common-steps/all/001_prefetch_registry_packages.sh.tpl"
	wrapperStep   = "candi/bashible/common-steps/all/075_install_systemctl_power_commands_wrapper.sh.tpl"
	packagesLib   = "candi/bashible/bashbooster/50_registrypackages.sh.tpl"

	firstMasterKubeletStep = "candi/bashible/common-steps/cluster-bootstrap/035_install_kubelet.sh.tpl"
)

var mandatoryPackages = []string{"jq", "yq", "curl", "which", "virt-what", "socat", "e2fsprogs", "iptables", "growpart", "lsblk", "nfs-mount"}

var prefetchedPlatformPackages = []string{"d8", "yq", "curl", "e2fsprogs", "iptables", "socat", "jq", "nfs-mount", "lsblk", "which", "growpart", "virt-what", "kubernetes-cni"}

func platformPackages(t *testing.T) map[string]string {
	t.Helper()
	built, ok := library.DefaultImagesDigests["registrypackages"].(map[string]interface{})
	require.True(t, ok, "testing/library/images_tags_generated.go lists no registrypackages")
	digests := map[string]any{}
	for key := range built {
		digests[key] = "sha256:" + key
	}
	doc, err := nodeletseed.FirstMaster(t.Context(), "candi", map[string]any{"registrypackages": digests}, []string{"http://127.0.0.1:5444"})
	require.NoError(t, err)

	packages := map[string]string{}
	for _, ext := range doc.Spec.Extensions {
		if ext.Name != "nodelet" {
			packages[ext.Name] = strings.TrimPrefix(ext.Digest, "sha256:")
		}
	}
	return packages
}

func stepContext(t *testing.T) map[string]any {
	t.Helper()
	packages := map[string]any{
		"containerd1736":     "sha256:containerd",
		"kubelet1356":        "sha256:kubelet",
		"crictl135":          "sha256:crictl",
		"registryProxy":      "sha256:registry-proxy",
		"kubernetesApiProxy": "sha256:kubernetes-api-proxy",
		"tomlMerge01":        "sha256:toml-merge",
		"pause":              "sha256:pause",
	}
	for name, key := range platformPackages(t) {
		packages[key] = "sha256:" + name
	}
	packages["nodelet"] = nodeletDigest
	context := bashibleContext("Normal", packages)
	context["kubernetesVersion"] = "1.35"
	context["k8s"] = map[string]any{"1.35": map[string]any{"patch": 6}}
	context["cri"] = "ContainerdV1"
	context["registry"] = map[string]any{"hosts": map[string]any{}}
	return context
}

func waitedFor(script string) []string {
	var names []string
	for _, line := range strings.Split(script, "\n") {
		args, found := strings.CutPrefix(strings.TrimSpace(line), "bb-package-wait ")
		if found {
			names = append(names, strings.Fields(args)...)
		}
	}
	return names
}

func TestTheStepsWaitForThePlatformPackagesTheAgentInstalls(t *testing.T) {
	chdirToRepositoryRoot(t)
	context := stepContext(t)

	for step, want := range map[string][]string{
		caStep:        {"d8-ca-updater"},
		mandatoryStep: mandatoryPackages,
		kubeletStep:   {"d8", "kubernetes-cni"},
		wrapperStep:   {"systemctl-power-commands-wrapper"},
	} {
		body := renderFile(t, step, context)
		require.Equal(t, want, waitedFor(body), step)
		for name := range platformPackages(t) {
			require.NotContains(t, body, fmt.Sprintf(`"%s:sha256:%s"`, name, name), "%s installs %s itself", step, name)
		}
		requireBashSyntax(t, body)
	}
	require.Contains(t, renderFile(t, kubeletStep, context), `rpp-get install --result "${kubelet_result}" "kubelet:sha256:kubelet"`)
}

func TestEveryPlatformPackageIsWaitedForOnce(t *testing.T) {
	chdirToRepositoryRoot(t)
	context := stepContext(t)

	var waited []string
	for _, step := range []string{caStep, mandatoryStep, kubeletStep, wrapperStep} {
		waited = append(waited, waitedFor(renderFile(t, step, context))...)
	}
	slices.Sort(waited)

	require.Equal(t, slices.Sorted(maps.Keys(platformPackages(t))), waited)
}

func TestThePrefetchLeavesTheAgentsPackagesToTheAgent(t *testing.T) {
	chdirToRepositoryRoot(t)

	body := renderFile(t, prefetchStep, stepContext(t))
	for _, name := range prefetchedPlatformPackages {
		require.NotContains(t, body, fmt.Sprintf(`\"%s:sha256:%s\"`, name, name))
	}
	require.Contains(t, body, `\"containerd:sha256:containerd\"`)
	require.Contains(t, body, `\"kubelet:sha256:kubelet\"`)
	requireBashSyntax(t, body)
}

func TestTheFirstMastersKubeletStepLeavesKubernetesCNIToTheAgent(t *testing.T) {
	chdirToRepositoryRoot(t)

	body := renderFile(t, firstMasterKubeletStep, stepContext(t))
	require.Equal(t, []string{"kubernetes-cni"}, waitedFor(body))
	require.NotContains(t, body, "kubernetes-cni:sha256:kubernetes-cni")
	require.NotContains(t, body, `__rpp_wait "kubernetes-cni"`)
	require.Contains(t, body, "rpp-get install \\\n  \"kubelet:sha256:kubelet\" \\\n  \"crictl:sha256:crictl\"\n")
	requireBashSyntax(t, body)
}

func recordPackage(t *testing.T, store, name, digest string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(store, name), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(store, name, "digest"), []byte(digest), 0o644))
}

func runPackageWait(t *testing.T, store string, names []string, env ...string) (string, error) {
	t.Helper()
	functions := renderFile(t, packagesLib, map[string]any{})
	script := "set -Eeuo pipefail\n" + functions +
		"\nsleep() { SECONDS=$((SECONDS + 3600)); }\nbb-package-wait " + strings.Join(names, " ") + "\n"
	cmd := exec.Command("bash", "-c", script)
	cmd.Env = append(os.Environ(), "BB_RP_INSTALLED_PACKAGES_STORE="+store)
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestPackageWaitTakesAnyDigestTheAgentRecorded(t *testing.T) {
	chdirToRepositoryRoot(t)
	store := t.TempDir()
	recordPackage(t, store, "jq", "sha256:whatever-the-agent-installed")

	out, err := runPackageWait(t, store, []string{"jq"})

	require.NoError(t, err, out)
}

func TestPackageWaitGivesUpNamingAMissingPackage(t *testing.T) {
	chdirToRepositoryRoot(t)
	store := t.TempDir()
	recordPackage(t, store, "jq", "sha256:whatever-the-agent-installed")

	out, err := runPackageWait(t, store, []string{"jq", "yq", "socat"})

	require.Error(t, err)
	require.Contains(t, out, "waiting for the node agent to install yq: journalctl -u nodelet")
}

func TestPackageWaitSendsItsWaitAsASpan(t *testing.T) {
	chdirToRepositoryRoot(t)
	store := t.TempDir()
	recordPackage(t, store, "jq", "sha256:whatever-the-agent-installed")
	recordPackage(t, store, "yq", "sha256:whatever-the-agent-installed")
	bin := t.TempDir()
	sent := filepath.Join(bin, "sent")
	require.NoError(t, os.WriteFile(filepath.Join(bin, "curl"), []byte("#!/bin/sh\necho \"$@\" >> "+sent+"\n"), 0o755))

	out, err := runPackageWait(t, store, []string{"jq", "yq"}, "DHCTL_TELEMETRY_ENABLED=true", "PATH="+bin+":"+os.Getenv("PATH"))

	require.NoError(t, err, out)
	request, err := os.ReadFile(sent)
	require.NoError(t, err)
	require.Contains(t, string(request), `"name":"bb-package-wait jq yq"`)
	require.Contains(t, string(request), "http://127.0.0.1:4318/v1/traces")
}

func TestTheFirstMasterSeedInstallsThePlatformPackages(t *testing.T) {
	chdirToRepositoryRoot(t)
	packages := map[string]any{"nodelet": nodeletDigest}
	for name, key := range platformPackages(t) {
		packages[key] = "sha256:" + name
	}
	seed, err := nodeletseed.FirstMaster(t.Context(), "candi", map[string]any{"registrypackages": packages}, []string{"http://127.0.0.1:5444"})
	require.NoError(t, err)
	want, err := json.Marshal(seed)
	require.NoError(t, err)

	for _, phase := range []string{prerequisitesPhase, installStep} {
		context := bashibleContext("ClusterBootstrap", packages)
		context["nodeletSeed"] = seed
		body := renderFile(t, phase, context)

		_, rest, found := strings.Cut(body, "--arg seed '")
		require.True(t, found, "%s: no seed in the script", phase)
		encoded, rest, found := strings.Cut(rest, "' \\\n    '")
		require.True(t, found, "%s: no jq filter after the seed", phase)
		filter, _, found := strings.Cut(rest, "'")
		require.True(t, found, "%s: the jq filter is not closed", phase)

		query, err := gojq.Parse(filter)
		require.NoError(t, err, phase)
		code, err := gojq.Compile(query, gojq.WithVariables([]string{"$name", "$seed"}))
		require.NoError(t, err, phase)
		written, ok := code.Run(nil, "master-0", encoded).Next()
		require.True(t, ok, phase)
		document, isObject := written.(map[string]any)
		require.True(t, isObject, "%s: the filter gave %v", phase, written)

		require.Equal(t, map[string]any{"name": "master-0"}, document["metadata"], phase)
		spec := document["spec"].(map[string]any)
		require.Equal(t, "master-0", spec["nodeName"], phase)
		delete(document, "metadata")
		delete(spec, "nodeName")
		got, err := json.Marshal(document)
		require.NoError(t, err)
		require.JSONEq(t, string(want), string(got), phase)
		requireBashSyntax(t, body)
	}
}
