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

package docs

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/spf13/afero"
)

// versionLinksFs is the filesystem a version site is published to. As Hugo
// writes a page of the version, the links of the page to the version
// (/<lang>/modules/<module>/<version>/x.html, with or without the language)
// become relative to the root of the version, ../x.html. The same files then
// serve the URL of the version, the URLs of the channels that point at it and
// the site without the language in the path. Links written by the templates
// go through it as well as those of the Markdown (_markup/render-link.html).
type versionLinksFs struct {
	afero.Fs
	public string // the publish directory: <public>/<lang>/modules/<module>/<version>/
	module string
	// version is the version as it is in the page paths: Hugo lower-cases
	// them.
	version string
	// own is the path of the version in a link, /modules/<module>/<version>/.
	own []byte
}

// ownLinkPattern is a link of a page to the version it belongs to; the
// languages, the module and the version are filled in. rewriteOwnLinks
// matches it without the regexp engine: a page is scanned for the path of the
// version with bytes.Index, 9% of a render with the regexp.
const ownLinkPattern = `(\b(?:href|src)=["'])(?:/(?:%s))?/modules/%s/%s/`

// VersionLinksRule names how the links of a version are rewritten. It is a
// part of the template revision (ondemand.TemplateRevision): a change of the
// rule renders the archives in S3 anew.
const VersionLinksRule = ownLinkPattern + " -> relative to the root of the version"

func newVersionLinksFs(fs afero.Fs, public, module, version string) *versionLinksFs {
	return &versionLinksFs{
		Fs:      fs,
		public:  filepath.Clean(public),
		module:  module,
		version: strings.ToLower(version),
		own:     []byte("/modules/" + module + "/" + strings.ToLower(version) + "/"),
	}
}

func (f *versionLinksFs) Create(name string) (afero.File, error) {
	file, err := f.Fs.Create(name)
	if err != nil {
		return nil, err
	}

	return f.wrap(name, file), nil
}

func (f *versionLinksFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	file, err := f.Fs.OpenFile(name, flag, perm)
	if err != nil || flag&(os.O_WRONLY|os.O_RDWR) == 0 {
		return file, err
	}

	return f.wrap(name, file), nil
}

// wrap rewrites the file on close when it is a page of the version.
func (f *versionLinksFs) wrap(name string, file afero.File) afero.File {
	root, ok := f.toRoot(name)
	if !ok {
		return file
	}

	return &versionLinksFile{File: file, own: f.own, root: root, buf: pageBufs.Get().(*bytes.Buffer)}
}

// toRoot returns the relative path from the page name to the root of the
// version: "./" for a page at the root, "../" one directory deeper.
func (f *versionLinksFs) toRoot(name string) (string, bool) {
	if filepath.Ext(name) != ".html" {
		return "", false
	}

	rel, err := filepath.Rel(f.public, filepath.Clean(name))
	if err != nil {
		return "", false
	}

	// <lang>/modules/<module>/<version>/<page path>
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) < 5 || parts[1] != "modules" || parts[2] != f.module || parts[3] != f.version {
		return "", false
	}

	if depth := len(parts) - 5; depth > 0 {
		return strings.Repeat("../", depth), true
	}

	return "./", true
}

// pageBufs and rewriteBufs keep the buffers of pages between writes: a
// version has hundreds of pages, a render rewrites them all.
var (
	pageBufs    = sync.Pool{New: func() any { return new(bytes.Buffer) }}
	rewriteBufs = sync.Pool{New: func() any { return new([]byte) }}
)

// versionLinksFile keeps what Hugo writes and writes it rewritten on close: a
// link may be split between two writes.
type versionLinksFile struct {
	afero.File
	own  []byte
	root string
	buf  *bytes.Buffer
}

func (f *versionLinksFile) Write(p []byte) (int, error) {
	if f.buf == nil {
		return 0, os.ErrClosed
	}
	return f.buf.Write(p)
}

func (f *versionLinksFile) WriteString(s string) (int, error) {
	if f.buf == nil {
		return 0, os.ErrClosed
	}
	return f.buf.WriteString(s)
}

// Close writes the page and gives its buffers back to the pools; dst and the
// page never share memory, they come from two pools.
func (f *versionLinksFile) Close() error {
	if f.buf == nil {
		return os.ErrClosed
	}
	dst := rewriteBufs.Get().(*[]byte)
	page, rewritten := rewriteOwnLinks(*dst, f.buf.Bytes(), f.own, f.root)
	_, err := f.File.Write(page)
	if closeErr := f.File.Close(); err == nil {
		err = closeErr
	}

	if rewritten {
		*dst = page[:0]
	}
	rewriteBufs.Put(dst)
	f.buf.Reset()
	pageBufs.Put(f.buf)
	f.buf = nil

	return err
}

// linkAttrs are the attributes ownLinkPattern rewrites, with their quotes.
var linkAttrs = [][]byte{[]byte(`href="`), []byte(`href='`), []byte(`src="`), []byte(`src='`)}

// rewriteOwnLinks does what ownLinkPattern with "${1}"+root does: a link
// attribute whose value starts with own, or /<lang> and own, gets root
// instead. The result goes to dst, reusing its memory, and the bool reports
// whether any link was rewritten; page is returned as is when it has no such
// link.
func rewriteOwnLinks(dst, page, own []byte, root string) ([]byte, bool) {
	out := dst[:0]
	found := false
	last := 0
	for i := 0; ; {
		j := bytes.Index(page[i:], own)
		if j < 0 {
			break
		}
		at := i + j
		i = at + 1
		start, ok := linkStart(page[last:at])
		if !ok {
			continue
		}
		if !found {
			found = true
			out = slices.Grow(out, len(page))
		}
		out = append(out, page[last:last+start]...)
		out = append(out, root...)
		last = at + len(own)
		i = last
	}
	if !found {
		return page, false
	}

	return append(out, page[last:]...), true
}

// linkStart reports where the path of a link ends in head, the text before
// own: right after a link attribute, or after one and /<lang>.
func linkStart(head []byte) (int, bool) {
	if n, ok := afterAttr(head); ok {
		return n, true
	}
	for _, lang := range Languages {
		if prefix, ok := bytes.CutSuffix(head, []byte("/"+lang)); ok {
			if n, ok := afterAttr(prefix); ok {
				return n, true
			}
		}
	}

	return 0, false
}

// afterAttr reports whether head ends with a link attribute that starts a
// word (\b), and its length.
func afterAttr(head []byte) (int, bool) {
	for _, attr := range linkAttrs {
		if !bytes.HasSuffix(head, attr) {
			continue
		}
		at := len(head) - len(attr)
		if at > 0 && isWordByte(head[at-1]) {
			return 0, false
		}
		return len(head), true
	}

	return 0, false
}

func isWordByte(b byte) bool {
	return b == '_' || '0' <= b && b <= '9' || 'a' <= b && b <= 'z' || 'A' <= b && b <= 'Z'
}
