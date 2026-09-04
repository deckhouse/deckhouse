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

// The registry this module runs: the upstream project as a dependency, plus the four things a
// cluster's registry needs that upstream has no place for.
//
// It used to be the upstream project copied into this repository — 205 files, based on a release
// upstream stopped maintaining, carrying nine patches and every security bump they imply. What is
// left here instead is a mapping of repository paths done outside the cache, a token request
// forwarded to a service on the loopback, a rule about whose word on a client's address is taken,
// and a second listener that accepts pushes. Everything else — the API, the storage, the garbage
// collection, TLS, the metrics — is `github.com/distribution/distribution/v3`, unmodified.
//
// The command line is deliberately the same as before, because the image's entrypoint and the
// syncer both name it: `registry serve <config>` and `registry garbage-collect <config>`.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/distribution/distribution/v3/configuration"

	"github.com/deckhouse/registry-distribution/internal/collect"
	"github.com/deckhouse/registry-distribution/internal/config"
	"github.com/deckhouse/registry-distribution/internal/serve"
)

// DefaultWrapperConfig is where this module's own configuration is looked for.
//
// Beside the registry's, written by the same pass of the syncer: the two describe one moment and
// must be replaced together.
const DefaultWrapperConfig = "/config/deckhouse.yaml"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	if len(arguments) == 0 {
		return usage()
	}

	command, rest := arguments[0], arguments[1:]

	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	wrapperPath := flags.String("deckhouse-config", DefaultWrapperConfig,
		"This module's own configuration: the upstream mapping, the token service, the write endpoint.")
	dryRun := flags.Bool("dry-run", false, "garbage-collect: report what would go without touching it.")
	removeUntagged := flags.Bool("delete-untagged", false,
		"garbage-collect: also remove manifests no tag names. Never on a pull-through cache, "+
			"where every manifest fetched on a miss is untagged by nature.")

	if err := flags.Parse(rest); err != nil {
		return err
	}

	positional := flags.Args()
	if len(positional) != 1 {
		return usage()
	}
	registryConfig := positional[0]

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	settings, err := parse(registryConfig)
	if err != nil {
		return err
	}

	switch command {
	case "serve":
		wrapper, err := config.Load(*wrapperPath)
		if err != nil {
			return err
		}
		return serve.Run(context.Background(), settings, wrapper, log)

	case "garbage-collect":
		// No wrapper configuration: a collection touches the store, not the network, and the one
		// thing this module's file would say about it — the upstream — is exactly what a collection
		// must not consult.
		return collect.Run(context.Background(), settings, collect.Options{
			DryRun:         *dryRun,
			RemoveUntagged: *removeUntagged,
		})

	default:
		return usage()
	}
}

func parse(path string) (*configuration.Configuration, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	settings, err := configuration.Parse(file)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	return settings, nil
}

func usage() error {
	return errors.New("usage: registry serve <config> | registry garbage-collect [--dry-run] <config>")
}
