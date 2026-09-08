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

// The OCI client in cr.go knows nothing about Deckhouse; this half knows where a module keeps
// its metadata inside the images (version.json, module.yaml, changelog.yaml,
// images_digests.json) and turns a release channel or an explicit tag into a version and the
// image digests. It stays free of the controller's dependency container and of the module
// definition type, because dhctl runs the same resolution before any cluster exists.

package cr

import (
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	crv1 "github.com/google/go-containerregistry/pkg/v1"
	"gopkg.in/yaml.v3"
)

// maxMetadataFileSize caps a single metadata file we read into memory. Release images are
// about a kilobyte and images_digests.json is a few dozen, so anything near this cap is a
// registry serving us something we did not ask for.
const maxMetadataFileSize = 4 << 20

// ReleaseInfo is what the release image (<repo>/<module>/release:<tag>) carries.
type ReleaseInfo struct {
	// Version is the "version" field of version.json, kept as an opaque string on purpose.
	// A development build ships {"version": "mr1"}, which is not a semver, and parsing it
	// here would break ResolveChannel exactly where it matters most - against a dev registry
	// at bootstrap time. Callers that need ordering parse it themselves.
	Version string

	// Digest of the release image itself. This is the marker the controller stores to tell
	// whether the channel has moved, not the digest of the module image.
	Digest string

	// Changelog is changelog.yaml decoded as-is; nil when the image carries none.
	Changelog map[string]any

	// ModuleYAML is module.yaml verbatim. It stays undecoded because the definition type
	// lives in the controller and drags addon-operator in with it.
	ModuleYAML []byte
}

// ResolveChannel reads <repo>/<module>/release:<tag> and returns its version.json contents.
// The tag is either a kebab-cased release channel (stable, early-access, rock-solid) or an
// explicit image tag - the release image is addressed identically in both cases, which is
// why there is one function and not two.
//
// The caller builds cli with NewClient(path.Join(repo, moduleName, "release"), opts...).
func ResolveChannel(ctx context.Context, cli Client, tag string) (ReleaseInfo, error) {
	var info ReleaseInfo

	img, err := cli.Image(ctx, tag)
	if err != nil {
		return info, fmt.Errorf("fetch image error: %w", err)
	}

	digest, err := img.Digest()
	if err != nil {
		return info, fmt.Errorf("fetch digest error: %w", err)
	}
	info.Digest = digest.String()

	files, err := extractFiles(img, "version.json", "changelog.yaml", "changelog.yml", "module.yaml")
	if err != nil {
		return info, fmt.Errorf("fetch release metadata error: %w", err)
	}

	if len(files["version.json"]) == 0 {
		return info, errors.New("metadata malformed: no version found")
	}

	var version struct {
		Version string `json:"version"`
	}
	if err = json.Unmarshal(files["version.json"], &version); err != nil {
		return info, fmt.Errorf("decode version.json: %w", err)
	}
	if version.Version == "" {
		return info, errors.New("metadata malformed: no version found")
	}
	info.Version = version.Version

	info.ModuleYAML = files["module.yaml"]

	changelog := files["changelog.yaml"]
	if len(changelog) == 0 {
		changelog = files["changelog.yml"]
	}
	if len(changelog) > 0 {
		// A malformed changelog must not fail the release: it is informational, and the
		// controller has always degraded to an empty one here.
		if err = yaml.Unmarshal(changelog, &info.Changelog); err != nil {
			info.Changelog = make(map[string]any)
		}
	}

	return info, nil
}

// ModuleImageTag turns a version into the module image tag the way the controller builds it:
// "v" + version. The version stays opaque, so the only thing we can safely do is avoid
// doubling the prefix when it is already there.
func ModuleImageTag(version string) string {
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}

// ImagesDigests reads the flat images_digests.json out of a module image
// (<repo>/<module>:<tag>). The map is flat name -> digest and lists every image the module
// build produced, including ones that are not shipped as files in the module image itself -
// that is how dhctl finds the terraform-manager bundle to pull.
//
// The caller builds cli with NewClient(path.Join(repo, moduleName), opts...).
func ImagesDigests(ctx context.Context, cli Client, tag string) (map[string]string, error) {
	img, err := cli.Image(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("fetch image error: %w", err)
	}

	files, err := extractFiles(img, "images_digests.json")
	if err != nil {
		return nil, fmt.Errorf("extract images_digests.json: %w", err)
	}
	if len(files["images_digests.json"]) == 0 {
		return nil, errors.New("images_digests.json not found in the module image")
	}

	digests := make(map[string]string)
	if err = json.Unmarshal(files["images_digests.json"], &digests); err != nil {
		return nil, fmt.Errorf("decode images_digests.json: %w", err)
	}

	return digests, nil
}

// extractFiles pulls the named root-level files out of the image's flattened filesystem.
// Names are matched case-insensitively, the way the controller has always matched them.
func extractFiles(img crv1.Image, names ...string) (map[string][]byte, error) {
	rc, err := Extract(img)
	if err != nil {
		return nil, fmt.Errorf("extract: %w", err)
	}
	defer rc.Close()

	wanted := make(map[string]struct{}, len(names))
	for _, name := range names {
		wanted[name] = struct{}{}
	}

	files := make(map[string][]byte, len(names))

	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return files, nil
		}
		// Read the header only after the error check: tar.Next returns a nil header
		// along with any error other than io.EOF.
		if err != nil {
			return nil, fmt.Errorf("tar reader next: %w", err)
		}

		// werf drops its service files next to the metadata; they are never metadata.
		if strings.HasPrefix(hdr.Name, ".werf") {
			continue
		}

		name := strings.ToLower(hdr.Name)
		if _, ok := wanted[name]; !ok {
			continue
		}

		// One byte past the cap, so a file that is too large is reported as such instead of
		// silently arriving truncated and failing later as a corrupt YAML/JSON document.
		content, err := io.ReadAll(io.LimitReader(tr, maxMetadataFileSize+1))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", hdr.Name, err)
		}
		if len(content) > maxMetadataFileSize {
			// A changelog is informational and can legitimately grow (a long-lived module
			// with a full per-version history), and the reader this replaces dropped one it
			// could not decode rather than failing the release - so an oversize one is
			// dropped too. version.json and images_digests.json are not optional: a
			// truncated one would be silently wrong, so there the cap stays hard.
			if strings.HasPrefix(name, "changelog.") {
				continue
			}
			return nil, fmt.Errorf("metadata file %s exceeds %d bytes", hdr.Name, maxMetadataFileSize)
		}
		files[name] = content
	}
}
