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

package registrydocs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/ondemand"
	"github.com/flant/docs-builder/internal/registry"
)

// Tags lists release tags in every source of the module. A source that can
// not be listed is skipped: some sources publish modules without a release
// repository.
func Tags(sources ModuleSources, clients *registry.Clients, logger *log.Logger) ondemand.TagLister {
	return func(ctx context.Context, module string) ([]string, error) {
		srcs, err := moduleSources(ctx, sources, module)
		if err != nil {
			return nil, err
		}

		var tags []string
		var errs []error
		for _, src := range srcs {
			client, err := clients.For(src)
			if err != nil {
				errs = append(errs, fmt.Errorf("source %s: %w", src.Name, err))
				continue
			}

			t, err := client.ListTags(ctx, module)
			if err != nil {
				logger.Warn("list release tags", slog.String("module", module), slog.String("module_source", src.Name), log.Err(err))
				errs = append(errs, fmt.Errorf("source %s: %w", src.Name, err))
				continue
			}

			tags = append(tags, t...)
		}

		failed := slices.ContainsFunc(errs, func(err error) bool { return !errors.Is(err, registry.ErrNotFound) })
		switch {
		case len(errs) == len(srcs) && !failed:
			return nil, fmt.Errorf("%w: no source has a release repository: %w", ondemand.ErrNoVersions, errors.Join(errs...))
		case len(errs) == len(srcs):
			return nil, errors.Join(errs...)
		case failed:
			// A source without a release repository is skipped; one that did
			// not answer may hold versions the others do not.
			return tags, fmt.Errorf("%w: %w", ondemand.ErrPartialList, errors.Join(errs...))
		}

		return tags, nil
	}
}
