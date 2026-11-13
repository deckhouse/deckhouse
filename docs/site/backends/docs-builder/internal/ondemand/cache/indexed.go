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

package cache

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/docs"
	"github.com/flant/docs-builder/internal/ondemand"
	"github.com/flant/docs-builder/internal/ondemand/atomicfile"
)

// indexDir holds the search index, llms.txt and the corpus of all modules,
// merged from the pinned versions (index): .index/<lang>/{search.json,llms.txt,corpus.json},
// the page of the list of modules (catalog) and indexedFile.
const indexDir = ".index"

// indexedFile keeps the targets the index was last built from across
// restarts, in indexDir and, with a cold cache, at the root of the bucket
// prefix: a new pod restores them from there. Targets do not depend on the
// template: an archive of an older one serves until it is rendered anew.
const indexedFile = "targets.json"

// indexedState is the content of indexedFile: the targets the index was
// built from, the versions of the release channels and the modules of the
// list of modules.
type indexedState struct {
	Targets  []ondemand.Target            `json:"targets"`
	Channels map[string]ondemand.Channels `json:"channels,omitempty"`
	// Modules are all the modules for the list of modules, including
	// those whose targets are not published. Absent in a file written
	// before it kept them: parseIndexed lists those of the targets.
	Modules []ondemand.Listed `json:"modules"`
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
	state.Targets = slices.DeleteFunc(state.Targets, func(t ondemand.Target) bool {
		return invalid("target", docs.ValidateModuleTag(t.Module, t.Version))
	})
	// A file written before it kept the modules (no modules, as opposed to
	// an empty list): those of the targets.
	if state.Modules == nil {
		state.Modules = listedOf(state.Targets)
	}
	state.Modules = slices.DeleteFunc(state.Modules, func(m ondemand.Listed) bool {
		return invalid("module", docs.ValidateModuleName(m.Module))
	})
	for module, chans := range state.Channels {
		if invalid("channels", docs.ValidateModuleName(module)) {
			delete(state.Channels, module)
			continue
		}
		for channel, version := range chans {
			err := docs.ValidateModuleTag(module, version)
			if err == nil && !slices.Contains(ondemand.SiteChannels, channel) {
				err = fmt.Errorf("unknown release channel %q of %s", channel, module)
			}
			if invalid("channel", err) {
				delete(chans, channel)
			}
		}
	}

	return state, nil
}

// listedOf lists the modules of the targets, for a targets.json written
// before it kept the modules. Whether they are enabled is not known until
// the leader writes them anew.
func listedOf(targets []ondemand.Target) []ondemand.Listed {
	out := make([]ondemand.Listed, 0, len(targets))
	for _, t := range targets {
		out = append(out, ondemand.Listed{Module: t.Module})
	}

	return out
}

// indexedTargets keeps indexedFile on the volume and, with a cold cache, in
// it, for the other pods and the next ones.
type indexedTargets struct {
	dir    string
	logger *log.Logger
	cold   indexCold
}

// indexCold is the cold cache as indexedTargets keeps targets.json there; no
// revision means no cold cache.
type indexCold interface {
	revision() string
	getList(ctx context.Context, key string) ([]byte, error)
	Put(ctx context.Context, key, path string) error
}

// save records the targets the index was built from, on the volume and in
// the cold cache.
func (ix *indexedTargets) save(ctx context.Context, state indexedState) error {
	path, err := ix.saveLocal(state)
	if err != nil {
		return err
	}

	if ix.cold.revision() != "" {
		if err := ix.cold.Put(ctx, indexedFile, path); err != nil {
			if ondemand.ColdMissing(err) {
				return errNotUploaded
			}
			return fmt.Errorf("put to S3: %w", err)
		}
	}

	return nil
}

// errNotUploaded means targets.json is on the volume only: the bucket is not
// ready yet.
var errNotUploaded = errors.New("the targets are not in S3 yet")

// saveLocal records the targets on the volume only, as a follower does: a
// pod that becomes the leader starts from them.
func (ix *indexedTargets) saveLocal(state indexedState) (string, error) {
	dir := filepath.Join(ix.dir, indexDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("mkdir index: %w", err)
	}

	path := filepath.Join(dir, indexedFile)
	if err := atomicfile.Write(path, func(w io.Writer) error { return json.NewEncoder(w).Encode(state) }); err != nil {
		return "", err
	}

	return path, nil
}

// load returns what save recorded, on the volume or, when the volume has
// none or a broken one, in the cold cache; nothing when there is none: the
// index is then built from scratch. A broken file in the cold cache counts
// as none: reading it again gives the same, and the followers can not read
// it either. The error means the cold cache did not answer: the caller must
// not record anything over the file until it is read.
func (ix *indexedTargets) load(ctx context.Context) (indexedState, error) {
	raw, err := os.ReadFile(filepath.Join(ix.dir, indexDir, indexedFile))
	if err == nil {
		state, err := parseIndexed(raw, ix.logger)
		if err == nil {
			return state, nil
		}
		ix.logger.Error("read indexed targets from the volume", log.Err(err))
	}
	if ix.cold.revision() == "" {
		return indexedState{}, nil
	}

	raw, err = ix.cold.getList(ctx, indexedFile)
	if ondemand.ColdMissing(err) {
		return indexedState{}, nil
	}
	if err != nil {
		return indexedState{}, fmt.Errorf("get indexed targets from S3: %w", err)
	}
	state, err := parseIndexed(raw, ix.logger)
	if err != nil {
		ix.logger.Error("read indexed targets from S3", log.Err(err))
		return indexedState{}, nil
	}

	return state, nil
}

// saveIndexed records the targets the index was built from (indexedTargets).
func (s *Store) saveIndexed(ctx context.Context, state indexedState) error {
	return s.indexed.save(ctx, state)
}

// saveIndexedLocal records the targets on the volume only (indexedTargets).
func (s *Store) saveIndexedLocal(state indexedState) (string, error) {
	return s.indexed.saveLocal(state)
}

// loadIndexed returns what saveIndexed recorded (indexedTargets).
func (s *Store) loadIndexed(ctx context.Context) (indexedState, error) {
	return s.indexed.load(ctx)
}
