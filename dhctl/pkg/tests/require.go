// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	// EnvCandiDir overrides the candi schemas directory used by tests that
	// would otherwise look at /deckhouse/candi (CI-prepared).
	EnvCandiDir = "DHCTL_TEST_CANDI_DIR"

	// EnvModulesDir overrides /deckhouse/modules for tests that read a
	// provider's candi tree straight from its module.
	EnvModulesDir = "DHCTL_TEST_MODULES_DIR"

	// EnvDhctlTestsDir overrides /dhctl-tests for cloud provider tests.
	EnvDhctlTestsDir = "DHCTL_TEST_DHCTL_TESTS_DIR"

	// EnvSkipDocker forces tests that require docker to skip even when docker is in path.
	EnvSkipDocker = "DHCTL_SKIP_DOCKER_TEST"

	// EnvSkipProvider forces infrastructureprovider tests to skip even when
	// the /dhctl-tests + /deckhouse trees are present.
	EnvSkipProvider = "DHCTL_SKIP_PROVIDER_TEST"

	// defaultCandiDir is the path where werf builds the cloud-providers
	// schemas in the CI test image.
	defaultCandiDir = "/deckhouse/candi"

	// defaultModulesDir is where werf puts the module trees (including each
	// provider's own candi) in the CI test image.
	defaultModulesDir = "/deckhouse/modules"

	// defaultDhctlTestsDir is the path CI populates with opentofu/terraform
	// binaries and provider plugins.
	defaultDhctlTestsDir = "/dhctl-tests"
)

// RequireDir skips the test when path is not a directory. Use it for fixtures
// that CI prepares (e.g. werf-built schema trees) but that may be missing on
// a developer machine — the skip message tells the developer what's missing.
func RequireDir(t *testing.T, path, reason string) {
	t.Helper()
	if isDir(path) {
		return
	}
	t.Skipf("%s missing (%s); skip", path, reason)
}

// RequireCandiDir skips the test when neither /deckhouse/candi nor a path
// pointed to by DHCTL_TEST_CANDI_DIR exists. Returns the resolved path so the
// test can use it directly.
func RequireCandiDir(t *testing.T) string {
	t.Helper()
	if dir := os.Getenv(EnvCandiDir); dir != "" {
		if isDir(dir) {
			return dir
		}
		t.Skipf("%s=%q is not a directory; skip", EnvCandiDir, dir)
	}
	if isDir(defaultCandiDir) {
		return defaultCandiDir
	}
	t.Skipf("candi schemas not found: %s missing and %s not set; skip", defaultCandiDir, EnvCandiDir)
	return ""
}

// ProviderModuleCandiDir returns the candi tree that lives inside a cloud
// provider module. External providers (dvp, yandex) are excluded from
// tools/build_includes/candi-cloud-providers-*.yaml — their candi bundle is
// delivered at runtime instead of being baked into the image — so the module
// tree is the only place their schemas and layouts can be read from in tests.
func ProviderModuleCandiDir(provider string) string {
	dir := os.Getenv(EnvModulesDir)
	if dir == "" {
		dir = defaultModulesDir
	}
	return filepath.Join(dir, "030-cloud-provider-"+strings.ToLower(provider), "candi")
}

// RequireProviderCandiDir resolves a single provider's candi tree, preferring
// the build-assembled candi/cloud-providers/<provider> and falling back to the
// provider module. Skips the test when neither is present. Returns the resolved
// path so the test can read schemas or layouts from it.
//
// A candidate counts only when it carries openapi/cluster_configuration.yaml:
// for an external provider the build bakes a partial tree into the image (just
// bashible, see externalCloudProviders in tools/build.go), so the directory
// existing is not enough. Prefer this over RequireDir on candi/cloud-providers
// for the same reason — that directory exists in the CI test image even when the
// provider a test needs is not in it.
func RequireProviderCandiDir(t *testing.T, provider string) string {
	t.Helper()
	provider = strings.ToLower(provider)

	candiDir := os.Getenv(EnvCandiDir)
	if candiDir == "" {
		candiDir = defaultCandiDir
	}

	candidates := []string{
		filepath.Join(candiDir, "cloud-providers", provider),
		ProviderModuleCandiDir(provider),
	}
	for _, dir := range candidates {
		if isFile(filepath.Join(dir, "openapi", "cluster_configuration.yaml")) {
			return dir
		}
	}

	t.Skipf("candi tree for provider %q not found in any of %v; skip", provider, candidates)
	return ""
}

// RequireDhctlTestsDir skips the test when neither /dhctl-tests nor a path
// pointed to by DHCTL_TEST_DHCTL_TESTS_DIR exists. Returns the resolved path.
func RequireDhctlTestsDir(t *testing.T) string {
	t.Helper()
	if dir := os.Getenv(EnvDhctlTestsDir); dir != "" {
		if isDir(dir) {
			return dir
		}
		t.Skipf("%s=%q is not a directory; skip", EnvDhctlTestsDir, dir)
	}
	if isDir(defaultDhctlTestsDir) {
		return defaultDhctlTestsDir
	}
	t.Skipf("test fixtures dir not found: %s missing and %s not set; skip", defaultDhctlTestsDir, EnvDhctlTestsDir)
	return ""
}

// RequireDocker skips the test when the docker CLI is not on PATH or the
// daemon is unreachable. DHCTL_SKIP_DOCKER_TEST=true forces a skip even when
// docker is available.
func RequireDocker(t *testing.T) {
	t.Helper()
	if os.Getenv(EnvSkipDocker) == "true" {
		t.Skipf("%s=true; skip", EnvSkipDocker)
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skipf("docker not on PATH (%v); skip", err)
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skipf("docker daemon not reachable (%v); skip", err)
	}
}

// RequireProviderEnv combines RequireDhctlTestsDir + RequireCandiDir +
// DHCTL_SKIP_PROVIDER_TEST opt-out into a single skip gate for the
// infrastructureprovider test suite.
func RequireProviderEnv(t *testing.T) {
	t.Helper()
	if os.Getenv(EnvSkipProvider) == "true" {
		t.Skipf("%s=true; skip", EnvSkipProvider)
	}
	RequireDhctlTestsDir(t)
	RequireCandiDir(t)
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
