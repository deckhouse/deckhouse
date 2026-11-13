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
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/docs"
)

// indexDir holds the search index, llms.txt and the corpus of all modules,
// merged from the pinned versions: .index/<lang>/{search.json,llms.txt,corpus.json},
// each with a gzip copy next to it for nginx (gzip_static): the search index
// of all modules takes megabytes and is loaded by the browser.
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
	Targets  []Target            `json:"targets"`
	Channels map[string]Channels `json:"channels,omitempty"`
	// Modules are all the modules for the list of modules, including
	// those whose targets are not published. Absent in a file written
	// before it kept them: parseIndexed lists those of the targets.
	Modules []Listed `json:"modules"`
}

// searchIndex is the search index, llms.txt and the corpus of all modules,
// built from the published versions of their targets, and targets.json, what
// they were built from.
type searchIndex struct {
	dir    string
	logger *log.Logger
	// atModuleRoot makes the links of the index lead to the module instead of
	// the version of its target (Config.IndexAtModuleRoot).
	atModuleRoot bool
	// publication tells how the store has the version of the target
	// published; stable is the version the stable channel of a module links
	// to.
	publication func(t Target) (published, bool)
	stable      func(module string) string
	// cold keeps targets.json for the other pods and the next ones.
	cold indexCold

	mu sync.Mutex
	// state names the published targets the index was last written from.
	state string
}

// indexCold is the cold cache as the index keeps targets.json there; no
// revision means no cold cache.
type indexCold interface {
	revision() string
	getList(ctx context.Context, key string) ([]byte, error)
	Put(ctx context.Context, key, path string) error
}

// WriteIndex merges the search indexes and the AI exports of the targets
// that are published into the files of indexDir. Each file is replaced
// atomically, so nginx serves either the old or the new one. No Hugo is
// involved: the versions already carry their parts.
//
// Nothing is written when no target is published (a registry outage at
// start): an empty index would hide the previous one. Nothing is written either when the published targets
// are the same as last time, so the files keep their Last-Modified.
func (ix *searchIndex) WriteIndex(targets []Target) error {
	dirs := make([]string, 0, len(targets))
	var state strings.Builder
	var toRoot []string
	for _, t := range targets {
		if v, ok := ix.publication(t); ok {
			dirs = append(dirs, filepath.Join(ix.dir, t.Module, t.Version))
			// Only the version of the stable channel is at the root of the
			// module; a module with no stable channel is found at its version.
			atRoot := ix.atModuleRoot && ix.stable(t.Module) == t.Version
			if atRoot {
				toRoot = append(toRoot, "/modules/"+t.Module+"/"+strings.ToLower(t.Version)+"/", "/modules/"+t.Module+"/")
			}
			fmt.Fprintf(&state, "%s/%s@%s:%t:%t;", t.Module, t.Version, v.rev, v.ai, atRoot)
		}
	}
	ix.mu.Lock()
	unchanged := state.String() == ix.state
	ix.mu.Unlock()

	if len(dirs) == 0 || unchanged {
		return nil
	}

	var errs []error
	for _, lang := range docs.Languages {
		parts := make([]string, 0, len(dirs))
		for _, dir := range dirs {
			parts = append(parts, filepath.Join(dir, lang))
		}

		out := filepath.Join(ix.dir, indexDir, lang)
		if err := os.MkdirAll(out, 0755); err != nil {
			return fmt.Errorf("mkdir index: %w", err)
		}

		for name, merge := range map[string]func([]string, io.Writer) error{
			"search.json": mergeSearch,
			"llms.txt":    mergeLLMs,
			"corpus.json": mergeCorpus,
		} {
			write := func(w io.Writer) error { return merge(parts, w) }
			// The site serves the target of a module at its root, and the
			// search and llms.txt lead there rather than to the version.
			if len(toRoot) > 0 {
				write = func(w io.Writer) error {
					var buf bytes.Buffer
					if err := merge(parts, &buf); err != nil {
						return err
					}
					_, err := strings.NewReplacer(toRoot...).WriteString(w, buf.String())
					return err
				}
			}
			if err := writeIndexFile(filepath.Join(out, name), write); err != nil {
				errs = append(errs, fmt.Errorf("%s %s: %w", lang, name, err))
			}
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	ix.mu.Lock()
	ix.state = state.String()
	ix.mu.Unlock()

	return nil
}

// saveIndexed records the targets the index was built from, on the volume
// and in the cold cache.
func (ix *searchIndex) saveIndexed(ctx context.Context, state indexedState) error {
	path, err := ix.saveIndexedLocal(state)
	if err != nil {
		return err
	}

	if ix.cold.revision() != "" {
		if err := ix.cold.Put(ctx, indexedFile, path); err != nil {
			if coldMissing(err) {
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

// saveIndexedLocal records the targets on the volume only, as a follower
// does: a pod that becomes the leader starts from them.
func (ix *searchIndex) saveIndexedLocal(state indexedState) (string, error) {
	dir := filepath.Join(ix.dir, indexDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("mkdir index: %w", err)
	}

	path := filepath.Join(dir, indexedFile)
	if err := writeFile(path, func(w io.Writer) error { return json.NewEncoder(w).Encode(state) }); err != nil {
		return "", err
	}

	return path, nil
}

// loadIndexed returns what saveIndexed recorded, on the volume or, when the
// volume has none or a broken one, in the cold cache; nothing when there is
// none: the index is then built from scratch. A broken file in the cold
// cache counts as none: reading it again gives the same, and the followers
// can not read it either. The error means the cold cache did not answer:
// the caller must not record anything over the file until it is read.
func (ix *searchIndex) loadIndexed(ctx context.Context) (indexedState, error) {
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
	if coldMissing(err) {
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

// WriteIndex merges the search indexes and the AI exports of the published
// targets into the files of indexDir (searchIndex).
func (s *Store) WriteIndex(targets []Target) error {
	return s.index.WriteIndex(targets)
}

// saveIndexed records the targets the index was built from (searchIndex).
func (s *Store) saveIndexed(ctx context.Context, state indexedState) error {
	return s.index.saveIndexed(ctx, state)
}

// saveIndexedLocal records the targets on the volume only (searchIndex).
func (s *Store) saveIndexedLocal(state indexedState) (string, error) {
	return s.index.saveIndexedLocal(state)
}

// loadIndexed returns what saveIndexed recorded (searchIndex).
func (s *Store) loadIndexed(ctx context.Context) (indexedState, error) {
	return s.index.loadIndexed(ctx)
}

// mergeSearch concatenates the documents and the parameters of the search
// indexes of the versions.
func mergeSearch(dirs []string, w io.Writer) error {
	merged := struct {
		Documents  []json.RawMessage `json:"documents"`
		Parameters []json.RawMessage `json:"parameters"`
	}{Documents: []json.RawMessage{}, Parameters: []json.RawMessage{}}

	for _, dir := range dirs {
		raw, err := os.ReadFile(filepath.Join(dir, "search.json"))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}

		var part struct {
			Documents  []json.RawMessage `json:"documents"`
			Parameters []json.RawMessage `json:"parameters"`
		}
		if err := json.Unmarshal(raw, &part); err != nil {
			return fmt.Errorf("%s: %w", dir, err)
		}

		merged.Documents = append(merged.Documents, part.Documents...)
		merged.Parameters = append(merged.Parameters, part.Parameters...)
	}

	return json.NewEncoder(w).Encode(merged)
}

// mergeLLMs keeps the preamble of the first llms.txt and appends the module
// sections ("## <module>") of every version.
func mergeLLMs(dirs []string, w io.Writer) error {
	header := false
	for _, dir := range dirs {
		raw, err := os.ReadFile(filepath.Join(dir, "llms.txt"))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}

		text := string(raw)
		sections := strings.Index(text, "\n## ")
		if sections < 0 {
			continue
		}

		if !header {
			if _, err := io.WriteString(w, text[:sections+1]); err != nil {
				return err
			}
			header = true
		}

		if _, err := io.WriteString(w, text[sections+1:]); err != nil {
			return err
		}
	}

	return nil
}

// mergeCorpus streams the documents of every corpus into one, with the other
// fields of the first one. A corpus is read one at a time: all of them
// together take hundreds of megabytes.
func mergeCorpus(dirs []string, w io.Writer) error {
	// A bufio.Writer keeps the first error of a write: Flush returns it.
	bw := bufio.NewWriter(w)
	started := false
	first := true

	for _, dir := range dirs {
		raw, err := os.ReadFile(filepath.Join(dir, "corpus.json"))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}

		var corpus map[string]json.RawMessage
		if err := json.Unmarshal(raw, &corpus); err != nil {
			return fmt.Errorf("%s: %w", dir, err)
		}

		var documents []json.RawMessage
		if raw, ok := corpus["documents"]; ok {
			if err := json.Unmarshal(raw, &documents); err != nil {
				return fmt.Errorf("%s documents: %w", dir, err)
			}
		}

		if !started {
			if err := writeCorpusHeader(bw, corpus); err != nil {
				return err
			}
			started = true
		}

		for _, d := range documents {
			if !first {
				_ = bw.WriteByte(',')
			}
			first = false
			_, _ = bw.Write(d)
		}
	}

	if !started {
		_, _ = bw.WriteString(`{"documents":[`)
	}
	_, _ = bw.WriteString("]}\n")

	return bw.Flush()
}

func writeCorpusHeader(w *bufio.Writer, corpus map[string]json.RawMessage) error {
	keys := make([]string, 0, len(corpus))
	for k := range corpus {
		if k != "documents" {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)

	// The last write returns the first error of any before it.
	_ = w.WriteByte('{')
	for _, k := range keys {
		name, err := json.Marshal(k)
		if err != nil {
			return err
		}
		_, _ = w.Write(name)
		_ = w.WriteByte(':')
		_, _ = w.Write(corpus[k])
		_ = w.WriteByte(',')
	}
	_, err := w.WriteString(`"documents":[`)

	return err
}

func gzipFile(path string, w io.Writer) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	gz := gzip.NewWriter(w)
	if _, err := io.Copy(gz, f); err != nil {
		return err
	}

	return gz.Close()
}

// writeIndexFile writes path and its gzip copy path.gz into temporary files
// and renames them into place one right after the other, so gzip clients and
// the others get the same content. Without the copy the file is still
// replaced: a stale copy would keep being served to gzip clients.
func writeIndexFile(path string, write func(io.Writer) error) error {
	plain, err := writeTemp(path, write)
	if err != nil {
		return err
	}
	defer os.Remove(plain)

	gz, gzErr := writeTemp(path+".gz", func(w io.Writer) error { return gzipFile(plain, w) })
	if gzErr == nil {
		defer os.Remove(gz)
		gzErr = os.Rename(gz, path+".gz")
	}
	if gzErr != nil {
		os.Remove(path + ".gz")
		gzErr = fmt.Errorf("gzip copy: %w", gzErr)
	}

	return errors.Join(os.Rename(plain, path), gzErr)
}

// writeFile replaces path at once with what write writes.
func writeFile(path string, write func(io.Writer) error) error {
	tmp, err := writeTemp(path, write)
	if err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}

	return nil
}

// writeTemp writes a temporary file next to path and returns its name.
func writeTemp(path string, write func(io.Writer) error) (string, error) {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-")
	if err != nil {
		return "", err
	}

	if err := write(f); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", err
	}

	// Served by nginx from another container.
	if err := os.Chmod(f.Name(), 0644); err != nil {
		os.Remove(f.Name())
		return "", err
	}

	return f.Name(), nil
}
