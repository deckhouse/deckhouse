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
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"

	validatev1 "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/validate/v1"
	"github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/server"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config/digests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/providerdir"
)

// stubValidatorEnv marks a copy of the test binary that was spawned to be a provider
// validator. A validator is a gRPC server the caller spawns and talks to, so a stub has
// to be one too: the binary this helper installs re-execs the test binary with the
// marker set, which serves the protocol for real without building a fixture binary.
const stubValidatorEnv = "D8_TEST_STUB_VALIDATOR"

// init turns a marked copy of the test binary into the validator it was spawned to be,
// before anything else in it gets a chance to run. An unmarked copy is the test binary
// itself and goes on as usual.
func init() {
	if os.Getenv(stubValidatorEnv) == "" {
		return
	}

	os.Exit(runStubValidator(os.Args[1:]))
}

// stubBundleDigest pins the stub bundle to a digest of this test process, so two packages sharing
// options.DefaultTmpDir() do not write into each other's bundle.
func stubBundleDigest() string {
	return fmt.Sprintf("sha256:%064x", os.Getpid())
}

// StubDeliveredProviderBundle lays provider's bundle out under downloadDir the way a real run
// leaves it — a digest-pinned directory holding the schema and the validator, with the default
// alias pointing at it — so dhctl takes it for delivered without a registry pull. Providers whose
// validator ships externally (e.g. yandex) need that binary; the schema is the real one, taken
// from RequireProviderCandiDir, i.e. from the provider module when the candi bundle is not baked
// into the image, so callers keep validating against the actual OpenAPI spec.
// The stub validator reports no violation at all, so anything that finds it passes every
// preflight check. Some callers have to place it under options.DefaultTmpDir(), the very
// directory a real dhctl run reads, because the code under test resolves the download dir
// itself. Leaving it behind would make the next real bootstrap or converge on this machine
// silently skip provider validation, so every artefact this helper creates is removed again
// through t.Cleanup, innermost first, and pre-existing paths are left untouched — an alias
// another test put there is moved aside and moved back rather than overwritten.
func StubDeliveredProviderBundle(t *testing.T, downloadDir, provider string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(downloadDir, 0o755))

	lockFile, err := os.OpenFile(filepath.Join(downloadDir, ".provider-bundle-test.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	require.NoError(t, err)
	require.NoError(t, syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX))
	t.Cleanup(func() {
		if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN); err != nil {
			t.Errorf("unlock stub provider bundle: %v", err)
		}
		if err := lockFile.Close(); err != nil {
			t.Errorf("close stub provider bundle lock: %v", err)
		}
	})

	provider = strings.ToLower(provider)

	candiDir := RequireProviderCandiDir(t, provider)

	digestDir := providerdir.ProviderDigestDir(downloadDir, provider, stubBundleDigest())
	openapiDir := filepath.Join(digestDir, "openapi")
	aliasPath := providerdir.ProviderDir(downloadDir, provider)

	// Remember which directories already existed: only the ones this helper creates may be
	// removed afterwards, and only once they are empty again.
	createdDirs := make([]string, 0, 2)
	for _, dir := range []string{digestDir, openapiDir} {
		if _, err := os.Lstat(dir); os.IsNotExist(err) {
			createdDirs = append(createdDirs, dir)
		}
	}

	require.NoError(t, os.MkdirAll(openapiDir, 0o755))

	schemaPath := filepath.Join(openapiDir, "cluster_configuration.yaml")
	validatorPath := filepath.Join(digestDir, "validator")

	// Another test may have left an alias of its own in a shared download dir; it is moved aside
	// for the duration rather than overwritten, so whatever it pointed at is still there after.
	backupPath := fmt.Sprintf("%s.dhctl-test-backup-%d", aliasPath, os.Getpid())
	_, aliasErr := os.Lstat(aliasPath)
	aliasExisted := aliasErr == nil
	if aliasExisted {
		require.NoError(t, os.Rename(aliasPath, backupPath))
	}
	require.NoError(t, os.Symlink(digestDir, aliasPath))

	t.Cleanup(func() {
		// Files first, then the alias, then the directories this helper created, deepest
		// last-created first. os.Remove on a directory only succeeds while it is empty, so a
		// directory another test still populates survives.
		for _, path := range []string{validatorPath, schemaPath} {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				t.Errorf("cleanup stub provider bundle: remove %s: %v", path, err)
			}
		}
		if err := os.Remove(aliasPath); err != nil && !os.IsNotExist(err) {
			t.Errorf("cleanup stub provider bundle: remove %s: %v", aliasPath, err)
		}
		if aliasExisted {
			if err := os.Rename(backupPath, aliasPath); err != nil {
				t.Errorf("cleanup stub provider bundle: restore %s: %v", aliasPath, err)
			}
		}
		for i := len(createdDirs) - 1; i >= 0; i-- {
			if err := os.Remove(createdDirs[i]); err != nil && !os.IsNotExist(err) {
				t.Logf("cleanup stub provider bundle: keep %s: %v", createdDirs[i], err)
			}
		}
	})

	schema, err := os.ReadFile(filepath.Join(candiDir, "openapi", "cluster_configuration.yaml"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(schemaPath, schema, 0o644))

	// A conformant validator is a gRPC server: the caller starts it with a subcommand of
	// its own, waits for the line announcing where it listens and calls it there (see
	// go_lib/dhctl-provider-protocol). A binary that goes down before announcing fails
	// closed, so the stub has to serve the protocol rather than print an answer.
	testBinary, err := os.Executable()
	require.NoError(t, err)

	validatorScript := fmt.Sprintf(
		"#!/bin/sh\n%s=1 exec %s \"$@\"\n",
		stubValidatorEnv, shellQuote(testBinary),
	)
	require.NoError(t, os.WriteFile(validatorPath, []byte(validatorScript), 0o755))

	stubEmbeddedBundleDigest(t, provider)
}

// stubEmbeddedBundleDigest points this installer's embedded digests at the bundle laid out above,
// so a caller that resolves the reference finds the directory already unpacked and needs neither a
// registry nor the fallback to get at it.
func stubEmbeddedBundleDigest(t *testing.T, provider string) {
	t.Helper()
	section := "cloudProvider" + strings.ToUpper(provider[:1]) + provider[1:]
	content := fmt.Sprintf(`{%q: {"terraformManager": %q}}`, section, stubBundleDigest())

	path := filepath.Join(t.TempDir(), "images_digests.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	t.Setenv(digests.ImagesDigestsFileEnv, path)
}

// runStubValidator is the whole life of a spawned copy: it reads the command line the
// way a real validator reads it and serves the protocol until it is asked to stop.
func runStubValidator(args []string) int {
	config, err := stubValidatorConfig(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)

		return 1
	}

	validator, err := server.Start(config, server.NewValidateService(stubValidator{}))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)

		return 1
	}

	stopped := make(chan os.Signal, 1)
	signal.Notify(stopped, syscall.SIGINT, syscall.SIGTERM)
	<-stopped

	if err := validator.Stop(); err != nil {
		fmt.Fprintln(os.Stderr, err)

		return 1
	}

	return 0
}

// stubValidatorConfig parses the argv the caller builds, so a change to either side of
// that contract shows up here instead of as a validator that never answers.
func stubValidatorConfig(args []string) (server.Config, error) {
	if len(args) == 0 || args[0] != server.ServeCommand {
		return server.Config{}, fmt.Errorf("stub validator: want the %s subcommand, got %q", server.ServeCommand, args)
	}

	flags := flag.NewFlagSet(server.ServeCommand, flag.ContinueOnError)
	configGetter := server.ConfigGetterFromFlags(flags)

	if err := flags.Parse(args[1:]); err != nil {
		return server.Config{}, err
	}

	config := configGetter()
	// The announce line is printed by the server itself; anything the stub logs on top of
	// it is noise in the output of the test that spawned it.
	config.Logger = slog.New(slog.DiscardHandler)

	return config, nil
}

// stubValidator answers every check with a clean result: a test that reaches the
// validator is testing what happens around it, not the provider's own checks.
type stubValidator struct{}

func (stubValidator) Validate(context.Context, validatev1.Input) (*validatev1.ValidateResponse, error) {
	return &validatev1.ValidateResponse{}, nil
}

// shellQuote makes a path safe to paste into the stub script, whatever it contains.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
