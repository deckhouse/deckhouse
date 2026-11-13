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
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gohugoio/hugo/common/hugo"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/flant/docs-builder/internal/docs"
	"github.com/flant/docs-builder/internal/metrics"
	"github.com/flant/docs-builder/internal/s3cache"
)

// ColdCache keeps archives of rendered versions outside the pod, so they
// survive a restart and are shared by replicas: an S3 bucket, be it the
// system storage of the cluster, a bucket of the user or the one of the
// public site.
//
// An object that is not there is an error wrapping ErrColdNotFound; a cold
// cache that is not set up yet answers every call with one wrapping
// ErrColdNotReady. The store takes either for an archive missing and renders
// the version; a follower takes only the first for an answer, and reads the
// registry itself while the cold cache is not ready.
type ColdCache interface {
	// Get writes the archive to w.
	Get(ctx context.Context, key string, w io.Writer) error
	Put(ctx context.Context, key, path string) error
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]ColdObject, error)
}

// The largest rendered version was 8.3 MB (main.go); anything far above it
// is not an archive of packVersion and is not let onto the cache volume.
const (
	maxColdArchive  = 64 << 20
	maxColdUnpacked = 256 << 20
)

var (
	errColdTooLarge = errors.New("the archive is too large for a version")
	errColdNoPages  = errors.New("the archive has no pages of a version")
	errNotInCold    = errors.New("the version is not in the cold cache")
	// errBadArchive marks a failure that is in the archive itself, as
	// opposed to the volume it is unpacked to.
	errBadArchive = errors.New("the archive is broken")
)

// archiveReader marks the read errors of an archive as errBadArchive, so
// they are told from the write errors of the volume.
type archiveReader struct{ r io.Reader }

func (a archiveReader) Read(p []byte) (int, error) {
	n, err := a.r.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		err = fmt.Errorf("%w: %w", errBadArchive, err)
	}

	return n, err
}

// limitedWriter fails once more than n bytes are written.
type limitedWriter struct {
	w io.Writer
	n int64
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > l.n {
		return 0, errColdTooLarge
	}
	l.n -= int64(len(p))
	return l.w.Write(p)
}

// coldArchives is the cold cache with the archives of rendered release
// versions in it: their keys under the template revision, packing them,
// uploading them in the background with retries, unpacking them back and
// evicting them. A development branch is never kept there.
type coldArchives struct {
	ColdCache

	// revisionOf is Config.Revision, branch is Config.IsBranch.
	revisionOf func() string
	branch     func(ctx context.Context, module, version string) (bool, error)
	// dir is the cache volume: the archives are packed and unpacked in its
	// tmp directory.
	dir string
	// timeout bounds an upload in the background (Config.RenderTimeout).
	timeout time.Duration
	logger  *log.Logger
	metrics *metricsstorage.MetricStorage // optional

	// background is the uploads running in the background. Nothing in the
	// pod waits for them, the tests do.
	background sync.WaitGroup
	// putPause is coldPutPause, shorter in the tests.
	putPause time.Duration
}

// revision is the revision of the archives in the cold cache, "" without
// one.
func (c *coldArchives) revision() string {
	if c.ColdCache == nil || c.revisionOf == nil {
		return ""
	}
	return c.revisionOf()
}

// isBranch reports whether a version without a revision is a development
// branch after all; when the cluster does not answer it is taken for one, so
// S3 is skipped rather than risked.
func (c *coldArchives) isBranch(ctx context.Context, module, version string) bool {
	if c.branch == nil {
		return false
	}
	branch, err := c.branch(ctx, module, version)
	if err != nil {
		c.logger.Warn("check for a module pull override, the version is not kept in S3", slog.String("module", module), slog.String("version", version), log.Err(err))
		return true
	}

	return branch
}

// coldKey is the archive of a version rendered by this template revision;
// pinned versions carry the AI export and are kept apart. The archives of a
// version sit together, so the one an older template rendered is found by
// listing coldVersionPrefix.
func coldKey(revision, module, version string, withAI bool) string {
	return coldVersionPrefix(module, version) + revision + coldSuffix(withAI)
}

func coldVersionPrefix(module, version string) string { return module + "/" + version + "/" }

func coldSuffix(withAI bool) string {
	if withAI {
		return coldAISuffix
	}

	return coldArchiveSuffix
}

const (
	coldArchiveSuffix = ".tar.gz"
	coldAISuffix      = ".ai.tar.gz"
)

// coldMode is how renderAndPublish uses the cold cache.
type coldMode int

const (
	// coldOrRender takes the archive, of any revision, or renders.
	coldOrRender coldMode = iota
	// coldOnly takes the archive, of any revision, and never renders.
	coldOnly
	// coldFresh takes an archive of this revision only and never renders.
	coldFresh
)

// staleMark marks a published version that came from an archive of another
// revision, so a restart keeps knowing it. It sits next to the language
// directories, out of what nginx serves.
const staleMark = ".stale"

// coldArchive is an archive key split into its parts.
type coldArchive struct {
	module, version, revision string
	withAI                    bool
}

// parseColdKey splits an archive key of coldKey; other objects (the list of
// indexed targets, the list of failed renders) are not archives. Neither is
// an object whose module or version is not named as one: they come from the
// bucket and become paths and registry references.
func parseColdKey(key string) (coldArchive, bool) {
	parts := strings.Split(key, "/")
	if len(parts) != 3 || !strings.HasSuffix(parts[2], coldArchiveSuffix) {
		return coldArchive{}, false
	}

	a := coldArchive{module: parts[0], version: parts[1]}
	a.revision, a.withAI = strings.CutSuffix(parts[2], coldAISuffix)
	if !a.withAI {
		a.revision = strings.TrimSuffix(parts[2], coldArchiveSuffix)
	}
	if a.revision == "" || docs.ValidateModuleTag(a.module, a.version) != nil {
		return coldArchive{}, false
	}

	return a, true
}

// fetch fills outDir from the cold cache and returns the key and then the
// archive it came from; the key is "" when there is none. A version rendered
// without the AI export may come from the archive of a pinned one, which has
// more. Without an archive of this revision the newest archive of another
// one is taken, so a new template does not make every version render at once:
// the bool reports it (stale), and the leader renders it anew in the
// background (Rerender).
//
// With mode coldFresh only an archive of this revision will do: a version
// served from an older one is replaced by it once the leader rendered it.
func (c *coldArchives) fetch(ctx context.Context, revision, module, version, outDir string, withAI bool, mode coldMode) (string, string, bool) {
	keys := []string{coldKey(revision, module, version, true)}
	if !withAI {
		keys = []string{coldKey(revision, module, version, false), keys[0]}
	}

	key, archive, missing := c.fetchFirst(ctx, keys, outDir)
	if key != "" || !missing || mode == coldFresh {
		return key, archive, false
	}

	objects, err := c.List(ctx, coldVersionPrefix(module, version))
	if err != nil {
		if !coldMissing(err) {
			c.logger.Warn("list versions in S3", slog.String("module", module), slog.String("version", version), log.Err(err))
		}
		return "", "", false
	}
	slices.SortFunc(objects, func(a, b ColdObject) int { return b.LastModified.Compare(a.LastModified) })
	keys = keys[:0]
	for _, o := range objects {
		if a, ok := parseColdKey(o.Key); ok && a.revision != revision && (a.withAI || !withAI) {
			keys = append(keys, o.Key)
		}
	}

	key, archive, _ = c.fetchFirst(ctx, keys, outDir)

	return key, archive, key != ""
}

// fetchFirst fills outDir from the first of the keys that has a usable
// archive and returns the key and the archive. The bool reports that none
// was there (missing), as opposed to a failure of the cold cache, which is
// not worth asking again.
func (c *coldArchives) fetchFirst(ctx context.Context, keys []string, outDir string) (string, string, bool) {
	for _, key := range keys {
		f, err := os.CreateTemp(filepath.Join(c.dir, tmpDir), "cold-*.tar.gz")
		if err != nil {
			c.logger.Warn("get version from S3", slog.String("key", key), log.Err(err))
			return "", "", false
		}
		err = c.Get(ctx, key, &limitedWriter{w: f, n: maxColdArchive})
		f.Close()
		if err != nil {
			os.Remove(f.Name())
			// Only a missing archive is worth looking for under another
			// key: an unavailable S3 would make the reader wait twice.
			if coldMissing(err) {
				continue
			}
			c.logger.Warn("get version from S3", slog.String("key", key), log.Err(err))
			return "", "", false
		}

		err = unpackVersion(f.Name(), outDir)
		if err == nil && !hasPages(outDir) {
			err = errColdNoPages
		}
		if err != nil {
			c.logger.Warn("unpack version from S3", slog.String("key", key), log.Err(err))
			os.Remove(f.Name())
			os.RemoveAll(outDir)
			// A broken archive would reach every reader of the version: it
			// goes, and the version is rendered and uploaded anew. A failure
			// of this pod's volume (a full disk) leaves the shared archive
			// alone: it may be the only one of an older template.
			if !errors.Is(err, errBadArchive) && !errors.Is(err, errColdTooLarge) && !errors.Is(err, errColdNoPages) {
				return "", "", false
			}
			if err := c.Delete(ctx, key); err != nil {
				c.logger.Warn("delete broken version from S3", slog.String("key", key), log.Err(err))
			}
			continue
		}

		return key, f.Name(), false
	}

	return "", "", true
}

// hasPages reports whether a version directory has the main page in any
// language, as every render does.
func hasPages(dir string) bool {
	for _, lang := range docs.Languages {
		for _, index := range indexPages {
			if _, err := os.Stat(filepath.Join(dir, lang, index)); err == nil {
				return true
			}
		}
	}

	return false
}

// snapshot hard-links the files of a rendered version into a private
// directory before it is published, so the reader does not wait for the
// packing. A render published over the version meanwhile cannot mix into the
// archive: published files are replaced, never written in place. It returns
// "" when the snapshot failed.
func (c *coldArchives) snapshot(outDir string) string {
	dir, err := os.MkdirTemp(filepath.Join(c.dir, tmpDir), "upload-")
	if err == nil {
		err = linkTree(outDir, dir)
	}
	if err != nil {
		os.RemoveAll(dir)
		c.logger.Warn("snapshot version for S3", log.Err(err))
		return ""
	}

	return dir
}

func linkTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if !d.Type().IsRegular() {
			return nil
		}

		return os.Link(path, target)
	})
}

// upload packs a snapshot of a published version and uploads it in the
// background, then removes both: the reader does not wait for it. A failed
// upload costs a reader's version a render after the next restart; a pinned
// one the leader uploads again on its next pass (uploadMissing), since the
// followers take it from the cold cache only.
func (c *coldArchives) upload(key, drop, snapshot string) {
	c.background.Add(1)
	go func() {
		defer c.background.Done()
		defer os.RemoveAll(snapshot)

		archive := c.pack(snapshot)
		if archive != "" {
			c.uploadArchive(key, drop, archive)
		}
	}()
}

// uploadAgain uploads the archive under key once more in the background and
// removes it.
func (c *coldArchives) uploadAgain(key, archive string) {
	c.background.Add(1)
	go func() {
		defer c.background.Done()
		c.uploadArchive(key, "", archive)
	}()
}

// pack packs a version directory into the tmp directory. It returns "" when
// packing failed.
func (c *coldArchives) pack(dir string) string {
	f, err := os.CreateTemp(filepath.Join(c.dir, tmpDir), "upload-*.tar.gz")
	if err != nil {
		c.logger.Warn("pack version for S3", log.Err(err))
		return ""
	}

	err = packVersion(dir, f)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(f.Name())
		c.logger.Warn("pack version for S3", log.Err(err))
		return ""
	}

	return f.Name()
}

// uploadArchive uploads the archive under key and removes it. drop is the
// key of an archive the new one covers: the archive of a pinned version
// holds the archive of a reader's version as well, so that one goes.
func (c *coldArchives) uploadArchive(key, drop, archive string) {
	defer os.Remove(archive)

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	err := c.putRetrying(ctx, key, archive)
	if err != nil && !coldMissing(err) && c.metrics != nil {
		c.metrics.CounterAdd(metrics.DocsBuilderOnDemandS3UploadFailures, 1, nil)
	}
	if err == nil && drop != "" {
		key = drop
		err = c.Delete(ctx, key)
	}
	// Before the bucket is ready every upload fails the same way.
	if err != nil && !coldMissing(err) {
		c.logger.Warn("put version to S3", slog.String("key", key), log.Err(err))
	}
}

// An upload to the cold cache is tried coldPutAttempts times before it
// counts as failed, coldPutPause apart and then longer: S3 fails a request
// now and then.
const (
	coldPutAttempts = 3
	coldPutPause    = 2 * time.Second
)

// putRetrying puts the archive under key, again while it fails,
// coldPutAttempts times at most. A bucket that is not ready is not asked
// again.
func (c *coldArchives) putRetrying(ctx context.Context, key, archive string) error {
	for attempt := 1; ; attempt++ {
		err := c.Put(ctx, key, archive)
		if err == nil || coldMissing(err) || attempt == coldPutAttempts {
			return err
		}
		select {
		case <-ctx.Done():
			return err
		case <-time.After(time.Duration(attempt) * c.putPause):
		}
	}
}

// uploadMissing uploads the archives of the pinned release versions this pod
// serves and the cold cache lacks, from the published pages. The followers
// take the versions targets.json names from the cold cache only: without it
// an upload that failed, a version taken for a branch while the cluster did
// not answer, or an archive evicted would stay missing on them for good. An
// archive of this template revision is enough, with the AI export when the
// version is served with it. A version served from an archive of an older
// revision is left to Rerender, which renders such archives anew; once the
// cold cache has none of an older revision of it, the version is marked lost
// and the pass renders it anew (renderPinned).
func (s *Store) uploadMissing(ctx context.Context, pinned []Target) error {
	revision := s.cold.revision()
	if revision == "" {
		return nil
	}
	objects, err := s.cold.List(ctx, "")
	if err != nil {
		return err
	}
	inCold := make(map[string]bool, len(objects))
	// older are the versions with an archive of another revision.
	older := make(map[string]bool)
	for _, o := range objects {
		inCold[o.Key] = true
		if a, ok := parseColdKey(o.Key); ok && a.revision != revision {
			older[key(a.module, a.version)] = true
		}
	}
	if err := os.MkdirAll(filepath.Join(s.cfg.Dir, tmpDir), 0700); err != nil {
		return fmt.Errorf("mkdir tmp: %w", err)
	}

	for _, t := range pinned {
		s.mu.Lock()
		v, ok := s.versions[key(t.Module, t.Version)]
		served := ok && !v.stale && v.rev == "" && t.Rev == ""
		withAI := ok && v.ai
		lost := ok && v.stale && !older[key(t.Module, t.Version)]
		marked := lost && !v.lost
		if ok {
			v.lost = lost
		}
		s.mu.Unlock()
		if marked {
			s.cfg.Logger.Info("a version served from an archive of an older template has none left in S3: it is rendered anew", slog.String("module", t.Module), slog.String("version", t.Version))
		}
		k := coldKey(revision, t.Module, t.Version, withAI)
		// The archive with the AI export holds the one without.
		if !served || inCold[k] || inCold[coldKey(revision, t.Module, t.Version, true)] || s.cold.isBranch(ctx, t.Module, t.Version) {
			continue
		}

		snapshot := s.cold.snapshot(filepath.Join(s.cfg.Dir, t.Module, t.Version))
		if snapshot == "" {
			continue
		}
		drop := ""
		if withAI {
			drop = coldKey(revision, t.Module, t.Version, false)
		}
		s.cfg.Logger.Info("upload a version missing from S3", slog.String("module", t.Module), slog.String("version", t.Version))
		s.cold.upload(k, drop, snapshot)
	}

	return nil
}

// packVersion writes the directory of a version as a tar.gz of its regular
// files and directories.
func packVersion(dir string, w io.Writer) error {
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil || rel == "." {
			return err
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() && !info.IsDir() {
			return nil
		}

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if info.IsDir() {
			hdr.Name += "/"
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)

		return err
	})
	if err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}

	return gz.Close()
}

// unpackVersion extracts an archive of packVersion into dir. Only regular
// files and directories inside dir are accepted: the archive comes from
// outside the pod.
func unpackVersion(archive, dir string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("%w: %w", errBadArchive, err)
	}
	tr := tar.NewReader(gz)

	var unpacked int64
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("%w: %w", errBadArchive, err)
		}

		name := filepath.FromSlash(hdr.Name)
		if !filepath.IsLocal(name) {
			return fmt.Errorf("%w: entry %q is outside the version", errBadArchive, hdr.Name)
		}
		path := filepath.Join(dir, name)
		if unpacked += hdr.Size; unpacked > maxColdUnpacked {
			return errColdTooLarge
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			if err != nil {
				return err
			}
			_, err = io.Copy(out, archiveReader{tr})
			if cerr := out.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: entry %q is not a file or a directory", errBadArchive, hdr.Name)
		}
	}

	return nil
}

// renderFormat is the part of the template revision that is in the code of
// docs-builder: it is raised by hand when the code starts writing the pages of
// a version differently (the layout of the archive, versionLinksFs beyond its
// rule, which is hashed itself), so the archives of the previous format are
// rendered anew, as after a change of the template. 2: the summary of the
// module (docs.SummaryFile) for the list of modules.
const renderFormat = "2"

// TemplateRevision names what renders a version: the site template, the
// Hugo embedded into docs-builder and renderFormat. A build of docs-builder
// that changes none of them keeps the archives in the cold cache; a new
// revision serves them until the leader renders them anew (Rerender).
func TemplateRevision(paths ...string) (string, error) {
	h := sha256.New()
	fmt.Fprintf(h, "hugo %s\x00format %s\x00links %s\x00", hugo.CurrentVersion, renderFormat, docs.VersionLinksRule)
	for _, root := range paths {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			fmt.Fprintf(h, "%s\x00", filepath.ToSlash(rel))
			_, err = io.Copy(h, f)

			return err
		})
		if err != nil {
			return "", err
		}
	}

	return hex.EncodeToString(h.Sum(nil))[:12], nil
}

// ColdObject is an archive in the cold cache: an object of the bucket, so
// the bucket is the cold cache as it is.
type ColdObject = s3cache.Object

// evict keeps the archives of the cold cache below high of its quota:
// above it, archives replaced by one of this template revision go first, then
// those of readers' versions, then those of pinned versions, the oldest first
// in each, until it is down to low. Pinned are the versions with the AI
// export and those targets.json names, the targets and the versions of their
// release channels: the followers take them from the cold cache only, while
// the pods serving them do not download them again, so their archives grow
// old in use. An archive of another revision that nothing replaces yet is
// still served (fetch) and goes with its kind. S3 does not track reads; a
// version restored from the cold cache is uploaded again, so the age is when
// a pod last needed it. Other objects (the list of indexed targets, the list
// of failed renders) are a few kilobytes and stay.
func (c *coldArchives) evict(ctx context.Context, quota int64, high, low float64) error {
	objects, err := c.List(ctx, "")
	if err != nil {
		return err
	}
	pinned, err := c.pinned(ctx)
	if err != nil {
		return err
	}
	revision := c.revision()

	type evictable struct {
		ColdObject
		class int
	}
	var archives []evictable
	var total int64
	current := make(map[string]bool)
	for _, o := range objects {
		a, ok := parseColdKey(o.Key)
		if !ok {
			// Shaped as an archive, yet not of a module version: it is
			// neither served nor evicted.
			if strings.Count(o.Key, "/") == 2 && strings.HasSuffix(o.Key, coldArchiveSuffix) {
				c.logger.Warn("skip an object of S3 that is not the archive of a module version", slog.String("key", o.Key))
			}
			continue
		}
		total += o.Size
		class := 1
		if a.withAI || pinned[key(a.module, a.version)] {
			class = 2
		}
		archives = append(archives, evictable{ColdObject: o, class: class})
		if a.revision == revision {
			current[coldVersionPrefix(a.module, a.version)+coldSuffix(a.withAI)] = true
		}
	}
	if float64(total) <= high*float64(quota) {
		return nil
	}

	for i, e := range archives {
		a, _ := parseColdKey(e.Key)
		prefix := coldVersionPrefix(a.module, a.version)
		// The archive of a pinned version covers the one of a reader's
		// version, as in a pass of the rerenderer.
		if a.revision != revision && (current[prefix+coldAISuffix] || !a.withAI && current[prefix+coldArchiveSuffix]) {
			archives[i].class = 0
		}
	}
	slices.SortFunc(archives, func(a, b evictable) int {
		if a.class != b.class {
			return a.class - b.class
		}
		return a.LastModified.Compare(b.LastModified)
	})

	removed := 0
	for _, e := range archives {
		if float64(total) <= low*float64(quota) {
			break
		}
		if err := c.Delete(ctx, e.Key); err != nil {
			return err
		}
		total -= e.Size
		removed++
	}

	c.logger.Info("evicted versions from S3", slog.Int("archives", removed), slog.Int64("bytes_left", total))

	return nil
}

// EvictCold keeps the archives of the cold cache below high of its quota
// (coldArchives).
func (s *Store) EvictCold(ctx context.Context, quota int64, high, low float64) error {
	return s.cold.evict(ctx, quota, high, low)
}
