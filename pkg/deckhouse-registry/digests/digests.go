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

// Package digests decodes images_digests.json, the file an artifact bundle
// ships to map each image it contains to a content-addressable digest.
//
// This package knows the file format and nothing else: it reads from a
// flattened image tar stream and has no dependency on the registry tree. The
// tree wires it in through BasicService.Digests, and each bundle declares where
// its own copy lives — see deckhouse.DigestsPath, deckhouse.InstallerDigestsPath,
// module.DigestsPath and packages.DigestsPath.
package digests

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
)

// FileName is the base name of the digests file. Where it sits inside an image
// depends on the bundle, so the full path is declared by each sub-tree package.
const FileName = "images_digests.json"

// maxBytes caps the digests file, guarding against a hostile bundle.
const maxBytes = 32 << 20 // 32 MiB

var (
	// ErrNotFound is returned when the image does not carry the file.
	ErrNotFound = errors.New("image carries no " + FileName)

	// ErrMixedShape is returned when the file is neither wholly flat nor wholly
	// nested, which means it matches neither known schema.
	ErrMixedShape = errors.New(FileName + " mixes flat and nested entries")
)

// Digests is the decoded content of an images_digests.json.
//
// The file comes in two shapes. A module, package or installer image ships a
// flat map of image name to digest, decoded into Images. The Deckhouse image
// ships one file covering every module it bundles — a map of module name to
// that same flat map — decoded into ByModule. Exactly one of the two is
// non-nil; IsNested says which.
type Digests struct {
	// Source is the in-image path the file was read from. It identifies which
	// kind of bundle the digests came from.
	Source string

	// Raw is the undecoded JSON, for consumers that apply their own schema.
	Raw []byte

	// Images maps image name to digest. Set for a flat file.
	Images map[string]string

	// ByModule maps module name to its image-name-to-digest map. Set for the
	// Deckhouse image. Module keys are CamelCase, as written by the build.
	ByModule map[string]map[string]string
}

// IsNested reports whether the file is keyed by module — true for the Deckhouse
// image, false for module, package and installer images.
func (d *Digests) IsNested() bool {
	return d.ByModule != nil
}

// Modules returns the module names a nested file covers, or nil for a flat one.
func (d *Digests) Modules() []string {
	if d.ByModule == nil {
		return nil
	}

	names := make([]string, 0, len(d.ByModule))
	for name := range d.ByModule {
		names = append(names, name)
	}

	return names
}

// Lookup returns the digest of one image. For a nested file module selects the
// module; for a flat file module must be empty.
func (d *Digests) Lookup(module, image string) (string, bool) {
	if d.IsNested() {
		digest, ok := d.ByModule[module][image]

		return digest, ok
	}

	if module != "" {
		return "", false
	}

	digest, ok := d.Images[image]

	return digest, ok
}

// Count returns the number of image entries, summed across modules for a nested
// file.
func (d *Digests) Count() int {
	if !d.IsNested() {
		return len(d.Images)
	}

	total := 0
	for _, images := range d.ByModule {
		total += len(images)
	}

	return total
}

// Read scans a flattened image tar for file and decodes it. file is relative to
// the image root; a leading "/" or "./" is accepted and normalized away.
// Returns ErrNotFound when the image does not carry it.
//
// mutate.Extract walks layers newest-first and emits each path once, so the
// first match is the effective version of the file and the scan stops there. An
// image without the file is read to the end before ErrNotFound is returned.
func Read(r io.Reader, file string) (*Digests, error) {
	wanted := normalize(file)
	reader := tar.NewReader(r)

	for {
		hdr, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%w at %s", ErrNotFound, wanted)
		}

		if err != nil {
			return nil, fmt.Errorf("read image tar: %w", err)
		}

		if normalize(hdr.Name) != wanted {
			continue
		}

		buf := bytes.NewBuffer(nil)

		written, err := io.Copy(buf, io.LimitReader(reader, maxBytes+1))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", wanted, err)
		}

		if written > maxBytes {
			return nil, fmt.Errorf("%s exceeds %d bytes", wanted, int64(maxBytes))
		}

		parsed, err := Parse(buf.Bytes())
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", wanted, err)
		}

		parsed.Source = wanted

		return parsed, nil
	}
}

// Parse decodes an images_digests.json, detecting whether it is flat or nested.
// Source is left empty; Read fills it in.
func Parse(raw []byte) (*Digests, error) {
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}

	parsed := &Digests{Raw: raw}

	// An empty file carries no shape information; treat it as flat so callers
	// never have to distinguish nil-flat from nil-nested.
	if len(entries) == 0 {
		parsed.Images = map[string]string{}

		return parsed, nil
	}

	nested, err := shapeIsNested(entries)
	if err != nil {
		return nil, err
	}

	if nested {
		parsed.ByModule = make(map[string]map[string]string, len(entries))

		for module, value := range entries {
			images := make(map[string]string)
			if err := json.Unmarshal(value, &images); err != nil {
				return nil, fmt.Errorf("decode images of module %q: %w", module, err)
			}

			parsed.ByModule[module] = images
		}

		return parsed, nil
	}

	parsed.Images = make(map[string]string, len(entries))

	for image, value := range entries {
		var digest string
		if err := json.Unmarshal(value, &digest); err != nil {
			return nil, fmt.Errorf("decode digest of image %q: %w", image, err)
		}

		parsed.Images[image] = digest
	}

	return parsed, nil
}

// shapeIsNested reports whether every entry is an object (the Deckhouse image's
// per-module layout) rather than a string (the flat layout). A file that mixes
// the two matches neither schema and is rejected.
func shapeIsNested(entries map[string]json.RawMessage) (bool, error) {
	var sawObject, sawString bool

	for _, value := range entries {
		switch firstToken(value) {
		case '{':
			sawObject = true
		case '"':
			sawString = true
		default:
			return false, fmt.Errorf("%w: unexpected entry %s", ErrMixedShape, string(value))
		}

		if sawObject && sawString {
			return false, ErrMixedShape
		}
	}

	return sawObject, nil
}

// firstToken returns the first non-whitespace byte of a JSON value, or 0.
func firstToken(value json.RawMessage) byte {
	for _, b := range value {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return b
		}
	}

	return 0
}

// normalize makes a tar entry name comparable to a declared path:
// "./deckhouse/x", "/deckhouse/x" and "deckhouse/x" all collapse to the last.
func normalize(name string) string {
	return strings.TrimPrefix(path.Clean("/"+name), "/")
}
