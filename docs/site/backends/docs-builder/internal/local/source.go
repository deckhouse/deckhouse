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

// Package local serves modules from directories on disk instead of a
// registry: the checkout of a module repository mounted into docs-builder
// on a developer's machine. A module has one version, rendered anew once
// its files change.
package local

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/docs"
	"github.com/flant/docs-builder/internal/ondemand"
	"github.com/flant/docs-builder/internal/registry"
)

// moduleFile marks a directory as a module repository and names the module.
const moduleFile = "module.yaml"

// archived are the files of a module repository the documentation is
// rendered from (see docs.RenderVersion): the tree of each directory and the
// separate files.
var (
	archivedDirs  = []string{"docs", "openapi", "crds"}
	archivedFiles = []string{
		moduleFile,
		"oss.yaml",
		"candi/openapi/cluster_configuration.yaml",
		"candi/openapi/doc-ru-cluster_configuration.yaml",
	}
)

// Source is the modules under a directory: every subdirectory with a
// module.yaml, or the subdirectories of those without one, so the directory
// is a module repository or a directory of them. The modules are listed
// anew on every call: a repository may be added while docs-builder runs.
type Source struct {
	dir     string
	version string
	logger  *log.Logger
}

// New returns the modules under dir, each as the one version given.
func New(dir, version string, logger *log.Logger) (*Source, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}
	// Every module gets this version: it must be a page path (ValidateModuleTag) and
	// a version of the site (ValidateModuleVersion, the address of the render status).
	if err := docs.ValidateModuleVersion("module", version); err != nil {
		return nil, err
	}

	return &Source{dir: dir, version: version, logger: logger}, nil
}

// module is a module repository under the directory.
type module struct {
	name string
	path string
}

// scan lists the module repositories, sorted by name. A directory whose
// module.yaml has no valid name is skipped with a warning, as is the second
// repository of a name.
func (s *Source) scan() ([]module, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", s.dir, err)
	}

	var paths []string
	for _, e := range entries {
		if !e.IsDir() || hidden(e.Name()) {
			continue
		}
		path := filepath.Join(s.dir, e.Name())
		if isModule(path) {
			paths = append(paths, path)
			continue
		}
		nested, err := os.ReadDir(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		for _, n := range nested {
			if n.IsDir() && !hidden(n.Name()) && isModule(filepath.Join(path, n.Name())) {
				paths = append(paths, filepath.Join(path, n.Name()))
			}
		}
	}

	seen := make(map[string]string, len(paths))
	modules := make([]module, 0, len(paths))
	for _, path := range paths {
		name, err := moduleName(path)
		if err == nil {
			err = docs.ValidateModuleName(name)
		}
		if err != nil {
			s.logger.Warn("skip a module directory", slog.String("path", path), log.Err(err))
			continue
		}
		if other, ok := seen[name]; ok {
			s.logger.Warn("skip a module directory: the module is in another one", slog.String("path", path), slog.String("module", name), slog.String("other", other))
			continue
		}
		seen[name] = path
		modules = append(modules, module{name: name, path: path})
	}
	slices.SortFunc(modules, func(a, b module) int { return strings.Compare(a.name, b.name) })

	return modules, nil
}

func (s *Source) find(name string) (module, error) {
	modules, err := s.scan()
	if err != nil {
		return module{}, err
	}
	for _, m := range modules {
		if m.name == name {
			return m, nil
		}
	}

	return module{}, fmt.Errorf("module %s: %w", name, ondemand.ErrNoVersions)
}

func isModule(path string) bool {
	info, err := os.Stat(filepath.Join(path, moduleFile))

	return err == nil && info.Mode().IsRegular()
}

func hidden(name string) bool {
	return strings.HasPrefix(name, ".")
}

func moduleName(path string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(path, moduleFile))
	if err != nil {
		return "", err
	}
	var meta struct {
		Name string `yaml:"name"`
	}
	if err := yaml.Unmarshal(raw, &meta); err != nil {
		return "", fmt.Errorf("parse %s: %w", moduleFile, err)
	}
	if meta.Name == "" {
		return "", fmt.Errorf("%s has no name", moduleFile)
	}

	return meta.Name, nil
}

// Modules lists the modules, each enabled with its version as the image of a
// development branch whose digest is the revision of its files: the Resolver
// (ondemand) then renders the version anew once the files change, as it does
// the branch of a ModulePullOverride.
func (s *Source) Modules(context.Context) ([]registry.Module, error) {
	revisions, err := s.revisions()
	if err != nil {
		return nil, err
	}

	out := make([]registry.Module, 0, len(revisions))
	for _, name := range slices.Sorted(maps.Keys(revisions)) {
		out = append(out, registry.Module{
			Name:     name,
			Enabled:  true,
			Override: &registry.Override{Tag: s.version, Digest: revisions[name]},
		})
	}

	return out, nil
}

// revisions are the revisions of the files of every module by name.
func (s *Source) revisions() (map[string]string, error) {
	modules, err := s.scan()
	if err != nil {
		return nil, err
	}

	out := make(map[string]string, len(modules))
	for _, m := range modules {
		rev, err := revision(m.path)
		if err != nil {
			return nil, fmt.Errorf("module %s: %w", m.name, err)
		}
		out[m.name] = rev
	}

	return out, nil
}

// revision changes when a file of the module is added, removed, renamed or
// written: it hashes the paths, the sizes and the modification times, not
// the contents, so a pass over the files costs a stat each.
func revision(dir string) (string, error) {
	h := sha256.New()
	err := walk(dir, func(rel string, info fs.FileInfo) error {
		fmt.Fprintf(h, "%s\x00%d\x00%d\x00", rel, info.Size(), info.ModTime().UnixNano())
		return nil
	})
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// walk calls fn for every regular file of the module that goes into the
// documentation archive, by its path relative to the module in slash form,
// in lexical order. Hidden files and directories are skipped.
func walk(dir string, fn func(rel string, info fs.FileInfo) error) error {
	for _, name := range archivedFiles {
		info, err := os.Stat(filepath.Join(dir, name))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if err := fn(name, info); err != nil {
			return err
		}
	}

	for _, sub := range archivedDirs {
		root := filepath.Join(dir, sub)
		if _, err := os.Stat(root); errors.Is(err, fs.ErrNotExist) {
			continue
		}
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if hidden(d.Name()) && path != root {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}

			return fn(filepath.ToSlash(rel), info)
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// writeArchive writes the documentation archive of the module repository in
// dir: a tar of its files as docs.RenderVersion reads them, the paths
// relative to the repository.
func writeArchive(w io.Writer, dir string) error {
	tw := tar.NewWriter(w)
	err := walk(dir, func(rel string, info fs.FileInfo) error {
		header := &tar.Header{Name: rel, Typeflag: tar.TypeReg, Mode: 0o644, Size: info.Size(), ModTime: info.ModTime()}
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("write header of %s: %w", rel, err)
		}
		f, err := os.Open(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			return err
		}
		defer f.Close()
		// A file written while it is archived: the size in the header is
		// what is copied, the next revision renders the rest.
		if _, err := io.CopyN(tw, f, info.Size()); err != nil {
			return fmt.Errorf("archive %s: %w", rel, err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return tw.Close()
}

// Tags lists the versions of the module: the one every local module has.
// A module that is not under the directory is unpublished
// (ondemand.ErrNoVersions).
func (s *Source) Tags(_ context.Context, name string) ([]string, error) {
	if _, err := s.find(name); err != nil {
		return nil, err
	}

	return []string{s.version}, nil
}

// Version is the version of the module: that of its stable channel, the
// latest one. A module that is not under the directory is unpublished.
func (s *Source) Version(_ context.Context, name string) (string, error) {
	if _, err := s.find(name); err != nil {
		return "", err
	}

	return s.version, nil
}

// Render renders the version of a module from its files with the site
// template in initDir (ondemand.RenderFunc). Any other version, or a module
// that is not under the directory, is ondemand.ErrVersionNotFound.
func (s *Source) Render(initDir string) ondemand.RenderFunc {
	return func(ctx context.Context, name, version, workDir, outDir string, withAI bool, stage func(ondemand.Stage)) error {
		m, err := s.find(name)
		if ondemand.Unpublished(err) || err == nil && version != s.version {
			return fmt.Errorf("%w: %s %s is not under %s", ondemand.ErrVersionNotFound, name, version, s.dir)
		}
		if err != nil {
			return err
		}

		stage(ondemand.StageRender)

		// The archive is streamed into the render; a render that stops
		// reading closes its end, which ends the writer.
		pr, pw := io.Pipe()
		defer pr.Close()
		go func() { pw.CloseWithError(writeArchive(pw, m.path)) }()

		return docs.RenderVersion(ctx, initDir, filepath.Join(workDir, "site"), outDir, name, version, pr, withAI, s.logger)
	}
}

// Watch calls changed when the files of a module change, or a module is
// added or removed, polling every interval until ctx is done. Polling: the
// directory is a bind mount, whose file events do not reach a container on
// every platform.
func (s *Source) Watch(ctx context.Context, every time.Duration, changed func()) {
	t := time.NewTicker(every)
	defer t.Stop()

	last, err := s.revisions()
	if err != nil {
		s.logger.Warn("watch the local modules", log.Err(err))
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}

		revisions, err := s.revisions()
		if err != nil {
			s.logger.Warn("watch the local modules", log.Err(err))
			continue
		}
		if maps.Equal(revisions, last) {
			continue
		}
		last = revisions
		s.logger.Info("the local modules changed, rendering them anew")
		changed()
	}
}
