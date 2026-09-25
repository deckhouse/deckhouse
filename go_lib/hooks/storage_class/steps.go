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

package storage_class

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/deckhouse/deckhouse/go_lib/regexpset"
)

// Append adds the entries of the source to the desired set.
func Append[T Namer](source Source[T]) Step[T] {
	return func(ctx context.Context, state *State[T]) error {
		classes, err := source(ctx, state)
		if err != nil {
			return err
		}

		state.Desired = append(state.Desired, classes...)

		return nil
	}
}

// OverrideByName merges the entries of the source into the desired set: an entry replaces the desired
// one with exactly the same name, in its place, and the rest are appended in the source order. A name
// the source lists more than once yields a single entry, the last one.
//
// Names are compared exactly, not as patterns: the source lists StorageClasses to create, and
// provider class names are often prefixes of each other (network-ssd and network-ssd-nonreplicated).
func OverrideByName[T Namer](source Source[T]) Step[T] {
	return func(ctx context.Context, state *State[T]) error {
		overrides, err := source(ctx, state)
		if err != nil {
			return err
		}

		byName := make(map[string]T, len(overrides))
		for _, class := range overrides {
			byName[class.GetName()] = class
		}

		merged := make([]T, 0, len(state.Desired)+len(overrides))
		for _, class := range state.Desired {
			name := class.GetName()
			if override, found := byName[name]; found {
				merged = append(merged, override)
				delete(byName, name)

				continue
			}

			merged = append(merged, class)
		}

		for _, class := range overrides {
			name := class.GetName()
			if override, pending := byName[name]; pending {
				merged = append(merged, override)
				delete(byName, name)
			}
		}

		state.Desired = merged

		return nil
	}
}

// Exclude drops the desired entries whose name fully matches one of the regular expressions listed at
// the values path. A missing or empty list excludes nothing.
func Exclude[T Namer](path string) Step[T] {
	return func(_ context.Context, state *State[T]) error {
		rawPatterns := state.Input.Values.Get(path).Array()
		if len(rawPatterns) == 0 {
			return nil
		}

		anchored := make([]string, 0, len(rawPatterns))
		for _, pattern := range rawPatterns {
			anchored = append(anchored, "^("+pattern.String()+")$")
		}

		excludes, err := regexpset.New(anchored...)
		if err != nil {
			return fmt.Errorf("compile %s: %w", path, err)
		}

		filtered := make([]T, 0, len(state.Desired))
		for _, class := range state.Desired {
			name := class.GetName()
			if excludes.Match(name) {
				state.Input.Logger.Info("Excluding storage class", slog.String("storage_class", name))

				continue
			}

			filtered = append(filtered, class)
		}

		state.Desired = filtered

		return nil
	}
}

// Filter keeps the desired entries the predicate accepts.
func Filter[T Namer](keep func(T) bool) Step[T] {
	return func(_ context.Context, state *State[T]) error {
		filtered := make([]T, 0, len(state.Desired))
		for _, class := range state.Desired {
			if keep(class) {
				filtered = append(filtered, class)
			}
		}

		state.Desired = filtered

		return nil
	}
}

// Map replaces every desired entry with what the function returns for it.
func Map[T Namer](transform func(T) T) Step[T] {
	return func(_ context.Context, state *State[T]) error {
		for i := range state.Desired {
			state.Desired[i] = transform(state.Desired[i])
		}

		return nil
	}
}

// SortByName orders the desired entries by name, keeping the relative order of equal names.
func SortByName[T Namer]() Step[T] {
	return func(_ context.Context, state *State[T]) error {
		sort.SliceStable(state.Desired, func(i, j int) bool {
			return state.Desired[i].GetName() < state.Desired[j].GetName()
		})

		return nil
	}
}

// SkipIf ends the run before anything is published when the predicate holds.
func SkipIf[T Namer](skip func(*State[T]) bool) Step[T] {
	return func(_ context.Context, state *State[T]) error {
		if skip(state) {
			return ErrStop
		}

		return nil
	}
}

// StopIfEmpty ends the run while the desired set is empty, leaving values and the cluster as they
// are — e.g. while the discovery data the classes come from has not arrived yet.
func StopIfEmpty[T Namer]() Step[T] {
	return func(_ context.Context, state *State[T]) error {
		if len(state.Desired) == 0 {
			return ErrStop
		}

		return nil
	}
}

// When runs the steps only when the predicate holds.
func When[T Namer](condition func(*State[T]) bool, steps ...Step[T]) Step[T] {
	return func(ctx context.Context, state *State[T]) error {
		if !condition(state) {
			return nil
		}

		for _, step := range steps {
			if err := step(ctx, state); err != nil {
				return err
			}
		}

		return nil
	}
}
