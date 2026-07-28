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

package release

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Metadata files a release image may carry, at the image root.
//
// version.json is the one every release image has, whatever it describes.
// The rest depend on the kind of release: a module release ships module.yaml,
// a package release ships package.yaml, and a Deckhouse release ships neither.
const (
	// VersionFile is the version and rollout metadata of the release.
	VersionFile = "version.json"
	// ChangelogFile is the human-facing changelog. ChangelogFileAlt is the
	// alternative spelling some builds emit.
	ChangelogFile    = "changelog.yaml"
	ChangelogFileAlt = "changelog.yml"
)

// The definition files — module.yaml and package.yaml — are named and mapped by
// package definition, which owns their schema. Read them through
// module.ReleaseService.Definition and packages.VersionService.Definition.

// maxBytes caps a single metadata file, guarding against a hostile release
// image. Release images are scratch images holding only metadata, so this is
// far above any legitimate size.
const maxBytes = 16 << 20 // 16 MiB

var (
	// ErrFileNotFound is returned when a release image does not carry a
	// requested metadata file.
	ErrFileNotFound = errors.New("release image does not carry file")

	// ErrNoVersionMetadata is returned when a release image carries no
	// version.json, and so declares no version.
	ErrNoVersionMetadata = errors.New("release image has no " + VersionFile)
)

// version.json has two schemas, and which one applies depends on what the
// release describes.
//
// A Deckhouse release drives a platform upgrade, so its file carries the
// rollout controls: canary waves, disruption notices, environment requirements
// and a suspend switch. See DeckhouseVersion.
//
// A module or package release drives nothing on its own — the platform decides
// when to apply it — so its file declares only the version. The manifest is a
// separate file. See PackageVersion.
//
// Only the version field is common to both, which is what Service.Version
// reads.

// DeckhouseVersion maps version.json of a Deckhouse release image.
type DeckhouseVersion struct {
	// Version is the semantic version the release publishes.
	Version string
	// Suspend marks a release that must not be rolled out.
	Suspend bool
	// Requirements are the constraints the release places on its environment,
	// keyed by requirement name (e.g. "k8s", "deckhouse").
	Requirements map[string]string
	// Disruptions lists disruptive changes, keyed by the minor version they
	// apply to.
	Disruptions map[string][]string
	// Canary holds per-channel canary rollout settings.
	Canary map[string]CanarySettings

	// Raw is the undecoded version.json, for consumers with their own schema.
	Raw []byte
}

// CanarySettings describes the canary rollout of a release on one channel.
type CanarySettings struct {
	Enabled  bool
	Waves    uint
	Interval time.Duration
}

// PackageVersion maps version.json of a module or package release image, whose
// only field is the version. The manifest it publishes travels beside it as
// module.yaml or package.yaml, not inside this file.
type PackageVersion struct {
	// Version is the semantic version the release publishes.
	Version string

	// Raw is the undecoded version.json, for consumers with their own schema.
	Raw []byte
}

// deckhouseVersionJSON mirrors the on-disk layout of a Deckhouse version.json.
type deckhouseVersionJSON struct {
	Version      string                    `json:"version"`
	Suspend      bool                      `json:"suspend"`
	Requirements map[string]string         `json:"requirements"`
	Disruptions  map[string][]string       `json:"disruptions"`
	Canary       map[string]canarySettings `json:"canary"`
}

type canarySettings struct {
	Enabled  bool     `json:"enabled"`
	Waves    uint     `json:"waves"`
	Interval Duration `json:"interval"`
}

// packageVersionJSON mirrors the on-disk layout of a module or package
// version.json.
type packageVersionJSON struct {
	Version string `json:"version"`
}

// commonVersionJSON is the one field both schemas share.
type commonVersionJSON struct {
	Version string `json:"version"`
}

// ParseDeckhouseVersion decodes the version.json of a Deckhouse release image.
func ParseDeckhouseVersion(raw []byte) (*DeckhouseVersion, error) {
	var decoded deckhouseVersionJSON
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("decode %s: %w", VersionFile, err)
	}

	version := &DeckhouseVersion{
		Version:      decoded.Version,
		Suspend:      decoded.Suspend,
		Requirements: decoded.Requirements,
		Disruptions:  decoded.Disruptions,
		Raw:          raw,
	}

	if len(decoded.Canary) > 0 {
		version.Canary = make(map[string]CanarySettings, len(decoded.Canary))
		for ch, settings := range decoded.Canary {
			version.Canary[ch] = CanarySettings{
				Enabled:  settings.Enabled,
				Waves:    settings.Waves,
				Interval: settings.Interval.Duration,
			}
		}
	}

	return version, nil
}

// ParsePackageVersion decodes the version.json of a module or package release
// image.
func ParsePackageVersion(raw []byte) (*PackageVersion, error) {
	var decoded packageVersionJSON
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("decode %s: %w", VersionFile, err)
	}

	return &PackageVersion{
		Version: decoded.Version,
		Raw:     raw,
	}, nil
}

// parseCommonVersion decodes just the version field, which both schemas share.
func parseCommonVersion(raw []byte) (string, error) {
	var decoded commonVersionJSON
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return "", fmt.Errorf("decode %s: %w", VersionFile, err)
	}

	return decoded.Version, nil
}

// ParseChangelog decodes a changelog.yaml.
func ParseChangelog(raw []byte) (map[string]any, error) {
	changelog := make(map[string]any)
	if err := yaml.Unmarshal(raw, &changelog); err != nil {
		return nil, fmt.Errorf("decode changelog: %w", err)
	}

	return changelog, nil
}

// readAll extracts every regular file from a flattened release image tar into a
// map keyed by normalized path. Release images are metadata-only, so reading
// one whole is cheap; each file is still capped at maxBytes against a hostile
// image. Reading everything, rather than a fixed set of names, lets the Release
// snapshot serve any file a build emits — including spellings this package does
// not enumerate.
func readAll(r io.Reader) (map[string][]byte, error) {
	files := make(map[string][]byte)
	reader := tar.NewReader(r)

	for {
		hdr, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return files, nil
		}

		if err != nil {
			return nil, fmt.Errorf("read release image tar: %w", err)
		}

		if hdr.FileInfo().IsDir() {
			continue
		}

		name := normalize(hdr.Name)

		buf := bytes.NewBuffer(nil)

		written, err := io.Copy(buf, io.LimitReader(reader, maxBytes+1))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}

		if written > maxBytes {
			return nil, fmt.Errorf("%s exceeds %d bytes", name, int64(maxBytes))
		}

		files[name] = buf.Bytes()
	}
}

// normalize makes a tar entry name comparable to a declared file name:
// "./version.json", "/version.json" and "version.json" all collapse to the last.
func normalize(name string) string {
	return strings.TrimPrefix(path.Clean("/"+name), "/")
}

// Duration decodes both "15m" strings and raw nanosecond numbers, matching how
// canary intervals are written into version.json across Deckhouse versions.
type Duration struct {
	time.Duration
}

// MarshalJSON writes the duration in its string form ("15m0s").
func (d Duration) MarshalJSON() ([]byte, error) {
	out, err := json.Marshal(d.Duration.String())
	if err != nil {
		return nil, fmt.Errorf("marshal duration: %w", err)
	}

	return out, nil
}

// UnmarshalJSON accepts a duration string or a number of nanoseconds.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var value any
	if err := json.Unmarshal(b, &value); err != nil {
		return fmt.Errorf("unmarshal duration: %w", err)
	}

	switch v := value.(type) {
	case float64:
		d.Duration = time.Duration(v)

		return nil
	case string:
		parsed, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("parse duration %q: %w", v, err)
		}

		d.Duration = parsed

		return nil
	default:
		return fmt.Errorf("invalid duration %s", string(b))
	}
}
