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

// StubDeliveredProviderBundle makes dhctl/pkg/config.providerCandiPresent treat
// provider's bundle as already delivered under downloadDir, without a
// registry pull. Providers whose validator ships externally (e.g. yandex)
// require a downloaded bundle — this fakes the validator binary and copies the
// real ClusterConfiguration schema alongside it, so callers keep validating
// against the actual OpenAPI spec instead of a synthetic one. The schema comes
// from RequireProviderCandiDir, i.e. from the provider module when the candi
// bundle is not baked into the image.
// The stub validator reports no violation at all, so anything that finds it passes every
// preflight check. Some callers have to place it under options.DefaultTmpDir(), the very
// directory a real dhctl run reads, because the code under test resolves the download dir
// itself. Leaving it behind would make the next real bootstrap or converge on this machine
// silently skip provider validation, so every artefact this helper creates is removed again
// through t.Cleanup, innermost first, and pre-existing paths are left untouched.
func StubDeliveredProviderBundle(t *testing.T, downloadDir, provider string) {
	t.Helper()
	provider = strings.ToLower(provider)

	candiDir := RequireProviderCandiDir(t, provider)

	providerDir := providerdir.ProviderDir(downloadDir, provider)
	openapiDir := filepath.Join(providerDir, "openapi")

	// Remember which directories already existed: only the ones this helper creates may be
	// removed afterwards, and only once they are empty again.
	createdDirs := make([]string, 0, 2)
	for _, dir := range []string{providerDir, openapiDir} {
		if _, err := os.Lstat(dir); os.IsNotExist(err) {
			createdDirs = append(createdDirs, dir)
		}
	}

	require.NoError(t, os.MkdirAll(openapiDir, 0o755))

	schemaPath := filepath.Join(openapiDir, "cluster_configuration.yaml")
	validatorPath := providerdir.ValidatorPath(downloadDir, provider)

	t.Cleanup(func() {
		// Files first, then the directories this helper created, deepest last-created first.
		// os.Remove on a directory only succeeds while it is empty, so a directory another
		// test still populates survives.
		for _, path := range []string{validatorPath, schemaPath} {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				t.Errorf("cleanup stub provider bundle: remove %s: %v", path, err)
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
