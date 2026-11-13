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

// Package index builds the search index, llms.txt and the corpus of all
// modules from the published versions of their targets.
package index

import (
	"bufio"
	"bytes"
	"compress/gzip"
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

	"github.com/flant/docs-builder/internal/docs"
	"github.com/flant/docs-builder/internal/ondemand"
	"github.com/flant/docs-builder/internal/ondemand/atomicfile"
)

// Config of an Index.
type Config struct {
	// Dir is the cache volume the versions are published in; Out is where
	// the files go, as <lang>/{search.json,llms.txt,corpus.json}, each with a
	// gzip copy next to it for nginx (gzip_static): the search index of all
	// modules takes megabytes and is loaded by the browser.
	Dir, Out string
	// AtModuleRoot makes the links of the index lead to the module instead of
	// the version of its target.
	AtModuleRoot bool
	// Publication tells how the store has the version of the target
	// published; Stable is the version the stable channel of a module links
	// to.
	Publication func(t ondemand.Target) (Published, bool)
	Stable      func(module string) string
}

// Published is how the store has a version published: the image digest of a
// development branch it was rendered from and whether with the AI export. A
// change of either writes the index anew.
type Published struct {
	Rev string
	AI  bool
}

// Index is the search index, llms.txt and the corpus of all modules, built
// from the published versions of their targets.
type Index struct {
	dir, out     string
	atModuleRoot bool
	publication  func(t ondemand.Target) (Published, bool)
	stable       func(module string) string

	mu sync.Mutex
	// state names the published targets the index was last written from.
	state string
}

// New makes the index of the store cfg describes.
func New(cfg Config) *Index {
	return &Index{
		dir:          cfg.Dir,
		out:          cfg.Out,
		atModuleRoot: cfg.AtModuleRoot,
		publication:  cfg.Publication,
		stable:       cfg.Stable,
	}
}

// WriteIndex merges the search indexes and the AI exports of the targets
// that are published into the files of Config.Out. Each file is replaced
// atomically, so nginx serves either the old or the new one. No Hugo is
// involved: the versions already carry their parts.
//
// Nothing is written when no target is published (a registry outage at
// start): an empty index would hide the previous one. Nothing is written either when the published targets
// are the same as last time, so the files keep their Last-Modified.
func (ix *Index) WriteIndex(targets []ondemand.Target) error {
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
			fmt.Fprintf(&state, "%s/%s@%s:%t:%t;", t.Module, t.Version, v.Rev, v.AI, atRoot)
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

		out := filepath.Join(ix.out, lang)
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
	plain, err := atomicfile.Temp(path, write)
	if err != nil {
		return err
	}
	defer os.Remove(plain)

	gz, gzErr := atomicfile.Temp(path+".gz", func(w io.Writer) error { return gzipFile(plain, w) })
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
