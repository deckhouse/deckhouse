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

// Package collect reclaims the blobs no manifest names any more.
//
// Its own implementation of the subcommand rather than upstream's, for one reason: upstream's takes
// the configuration file and builds the storage from it, and this module's configuration file names
// a pull-through cache. A collection must never run against one — the reachable set would be
// computed from what the cache can FETCH rather than from what it holds — so the storage is opened
// here directly, from the same directory and with no proxy in front of it.
//
// The decisions above it belong to the syncer: which tags may go, whether the run is allowed at all,
// and what to report. This is the sweep those decisions end in.
package collect

import (
	"context"
	"fmt"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry/storage"
	"github.com/distribution/distribution/v3/registry/storage/driver/factory"
)

// Options are the two questions a sweep asks.
type Options struct {
	// DryRun reports what would go without touching anything.
	DryRun bool

	// RemoveUntagged is deliberately not the default, and the syncer never asks for it: in a
	// pull-through cache every manifest fetched on a miss is untagged by nature, so removing the
	// untagged ones empties the store of exactly the images it was asked to hold — a garbage
	// collection turned into a cache flush. What a run reclaims is what deleted tags made
	// unreachable.
	RemoveUntagged bool
}

// Run opens the store and sweeps it.
func Run(ctx context.Context, settings *configuration.Configuration, options Options) error {
	driverName := settings.Storage.Type()
	if driverName == "" {
		return fmt.Errorf("the configuration names no storage driver")
	}

	parameters := settings.Storage.Parameters()
	if parameters == nil {
		parameters = configuration.Parameters{}
	}

	driver, err := factory.Create(ctx, driverName, parameters)
	if err != nil {
		return fmt.Errorf("opening the %s storage: %w", driverName, err)
	}

	// Built from the driver alone: no proxy, no middleware, no cache. What is reachable is what the
	// files say, which is the only thing a collection may act on.
	namespace, err := storage.NewRegistry(ctx, driver)
	if err != nil {
		return fmt.Errorf("reading the store: %w", err)
	}

	if err := storage.MarkAndSweep(ctx, driver, namespace, storage.GCOpts{
		DryRun:         options.DryRun,
		RemoveUntagged: options.RemoveUntagged,
	}); err != nil {
		return fmt.Errorf("reclaiming blobs: %w", err)
	}

	return nil
}
