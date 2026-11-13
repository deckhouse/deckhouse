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

package ondemand

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/docs"
)

// maxColdList bounds a list kept in the cold cache (targets.json, the failed
// renders): a few kilobytes in practice.
const maxColdList = 1 << 20

// coldMissing reports whether err means the object is not in the cold cache,
// or the cache is not set up yet.
func (s *Store) coldMissing(err error) bool {
	return s.cfg.ColdNotFound != nil && s.cfg.ColdNotFound(err)
}

// coldGetList reads a list kept in the cold cache.
func (s *Store) coldGetList(ctx context.Context, key string) ([]byte, error) {
	var buf bytes.Buffer
	if err := s.cfg.Cold.Get(ctx, key, &limitedWriter{w: &buf, n: maxColdList}); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// parseIndexed reads targets.json. A list of targets is what the files of
// the cluster had before the channels. The file comes from outside the pod,
// and its modules, versions and channels become paths and registry
// references: an entry that is not named as one is left out with a warning.
func parseIndexed(raw []byte, logger *log.Logger) (indexedState, error) {
	var state indexedState
	into := any(&state)
	if trimmed := bytes.TrimSpace(raw); len(trimmed) > 0 && trimmed[0] == '[' {
		into = &state.Targets
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return indexedState{}, err
	}

	invalid := func(entry string, err error) bool {
		if err != nil {
			logger.Warn("skip an entry of "+indexedFile, slog.String("entry", entry), log.Err(err))
		}
		return err != nil
	}
	state.Targets = slices.DeleteFunc(state.Targets, func(t Target) bool {
		return invalid("target", docs.ValidateModuleTag(t.Module, t.Version))
	})
	// A file written before it kept the modules (no modules, as opposed to
	// an empty list): those of the targets.
	if state.Modules == nil {
		state.Modules = listedOf(state.Targets)
	}
	state.Modules = slices.DeleteFunc(state.Modules, func(m Listed) bool {
		return invalid("module", docs.ValidateModuleName(m.Module))
	})
	for module, chans := range state.Channels {
		if invalid("channels", docs.ValidateModuleName(module)) {
			delete(state.Channels, module)
			continue
		}
		for channel, version := range chans {
			err := docs.ValidateModuleTag(module, version)
			if err == nil && !slices.Contains(siteChannels, channel) {
				err = fmt.Errorf("unknown release channel %q of %s", channel, module)
			}
			if invalid("channel", err) {
				delete(chans, channel)
			}
		}
	}

	return state, nil
}

// coldPinned returns the versions targets.json names, the targets and the
// versions of their release channels, by key.
func (s *Store) coldPinned(ctx context.Context) (map[string]bool, error) {
	raw, err := s.coldGetList(ctx, indexedFile)
	if s.coldMissing(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get the targets from S3: %w", err)
	}
	// The followers can not read a broken file either: they wait for no
	// archive.
	state, err := parseIndexed(raw, s.cfg.Logger)
	if err != nil {
		return nil, nil
	}

	pinned := make(map[string]bool)
	for _, t := range state.Targets {
		pinned[key(t.Module, t.Version)] = true
	}
	for module, chans := range state.Channels {
		for _, version := range chans {
			pinned[key(module, version)] = true
		}
	}

	return pinned, nil
}

// restoreAll publishes from the cold cache the targets, with the AI export,
// and the versions of their channels, and links the channels. It returns
// how many of them are not served and whether they are served from this
// template revision too (fresh), the targets with the AI export: a version
// missing, served from an archive of an older revision, or a target served
// without the export, which may have failed on the leader, is looked for
// again on the next call. A development branch is never in the cold cache
// and counts for neither.
// ponytail: a release version restored here keeps being served when a
// ModulePullOverride later pulls the same tag (a tag shaped vX.Y.Z), until
// the pod restarts; unpublish it here if such overrides become common.
func (s *Store) restoreAll(ctx context.Context, targets []Target, chans map[string]Channels, logger *log.Logger) (int, bool) {
	chanTargets := channelTargets(byModule(targets), chans)
	s.Pin(targets, chanTargets)

	missing, fresh := 0, true
	restore := func(t Target, withAI bool) {
		ok, err := s.restorePinned(ctx, t, withAI)
		if err != nil && ctx.Err() == nil {
			logger.Warn("restore a version from S3", slog.String("module", t.Module), slog.String("version", t.Version), log.Err(err))
		}
		if t.Rev != "" {
			return
		}
		if !ok {
			missing++
		}
		fresh = fresh && s.publishedAsWanted(t, withAI)
	}
	for _, t := range targets {
		restore(t, true)
	}
	for _, t := range chanTargets {
		restore(t, false)
	}
	if ctx.Err() != nil {
		return missing, false
	}

	s.SetChannels(chans)

	return missing, fresh
}
