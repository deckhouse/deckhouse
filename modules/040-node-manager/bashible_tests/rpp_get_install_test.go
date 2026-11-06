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

package bashible_tests

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

const bashibleLib = "candi/bashible/lib.sh.tpl"

// What a node runs is a script that leaves a marker, so the test sees whether it ran.
const (
	genuineRppGet     = "#!/bin/sh\ntouch \"$RPP_GET_MARKER\"\n"
	substitutedRppGet = "#!/bin/sh\ntouch \"$RPP_GET_MARKER.substituted\"\n"
)

func sha256Digest(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func imageLayer(t *testing.T, files map[string]string) []byte {
	t.Helper()

	contents := map[string][]byte{}
	for name, file := range files {
		contents[name] = []byte(file)
	}

	layer, err := crane.Layer(contents)
	require.NoError(t, err)
	compressed, err := layer.Compressed()
	require.NoError(t, err)
	content, err := io.ReadAll(compressed)
	require.NoError(t, err)
	return content
}

// The last layer of the rpp-get image holds the binary beside the install and uninstall scripts.
func rppGetLayer(t *testing.T, binary string) []byte {
	t.Helper()
	return imageLayer(t, map[string]string{"install": "#!/bin/sh\n", "uninstall": "#!/bin/sh\n", "rpp-get": binary})
}

// The manifest of an image as registrypackages builds rpp-get: the digest the bundle carries is
// the manifest's, the manifest names the layers, an empty one first and the one with the binary last.
func rppGetManifest(t *testing.T, layers ...[]byte) []byte {
	t.Helper()
	descriptors := []map[string]any{}
	for _, layer := range layers {
		descriptors = append(descriptors, map[string]any{"mediaType": "application/vnd.oci.image.layer.v1.tar+gzip", "digest": sha256Digest(layer), "size": len(layer)})
	}
	manifest, err := json.MarshalIndent(map[string]any{
		"schemaVersion": 2,
		"mediaType":     "application/vnd.oci.image.manifest.v1+json",
		"config":        map[string]any{"mediaType": "application/vnd.oci.image.config.v1+json", "digest": sha256Digest([]byte("{}")), "size": 2},
		"layers":        descriptors,
	}, "", "  ")
	require.NoError(t, err)
	return manifest
}

// A proxy on a master's bootstrap port. A missing manifest or layer answers 404, as a proxy of an
// earlier release does. The bare path serves the substituted binary, so a node that falls back to
// it leaves a marker.
func serveRppGet(t *testing.T, manifest, layer []byte) string {
	t.Helper()
	mux := http.NewServeMux()
	for path, body := range map[string][]byte{
		"/cluster-uuid/rpp-get/manifest": manifest,
		"/cluster-uuid/rpp-get/layer":    layer,
		"/cluster-uuid/rpp-get":          []byte(substitutedRppGet),
	} {
		if body == nil {
			continue
		}
		mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) })
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return strings.TrimPrefix(srv.URL, "http://")
}

func renderRppGetLib(t *testing.T, digest string) string {
	t.Helper()
	content, err := os.ReadFile(bashibleLib)
	require.NoError(t, err)
	content = append(content, []byte("\n{{ template \"bb-minget\" . }}\n")...)
	rendered, err := template.RenderTemplate(filepath.Base(bashibleLib), content, bashibleContext("Normal", map[string]any{"rppGet": digest}))
	require.NoError(t, err)
	return rendered.Content.String()
}

// d8-curl is not on a test machine, and macOS has no sha256sum. A sha256sum that fails on the
// layer stands for a node where hashing it fails.
func rppGetStubs(t *testing.T, dir string, layerHashFails bool) string {
	t.Helper()
	stubs := filepath.Join(dir, "stubs")
	require.NoError(t, os.MkdirAll(stubs, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stubs, "d8-curl"), []byte("#!/bin/sh\nexec curl \"$@\"\n"), 0o755))
	sha256sum := "sha256sum"
	if _, err := exec.LookPath(sha256sum); err != nil {
		sha256sum = "shasum -a 256"
	}
	failOnLayer := ""
	if layerHashFails {
		failOnLayer = "case \"$1\" in */layer) exit 1 ;; esac\n"
	}
	stub := "#!/bin/sh\nPATH=\"${PATH#*:}\"\n" + failOnLayer + "exec " + sha256sum + " \"$@\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(stubs, "sha256sum"), []byte(stub), 0o755))
	return stubs
}

func runRppGetInstall(t *testing.T, dir, lib, proxyAddresses string, layerHashFails bool) (string, error) {
	t.Helper()
	bin := filepath.Join(dir, "rpp-get")
	script := strings.ReplaceAll(lib, "/opt/deckhouse/bin/rpp-get", bin) + "\nsleep() { :; }\nbb-rpp-get-install 1\n"
	cmd := exec.Command("bash", "-c", "set -Eeo pipefail\n"+script)
	cmd.Env = append(os.Environ(),
		"PATH="+rppGetStubs(t, dir, layerHashFails)+":"+os.Getenv("PATH"),
		"BB_RP_INSTALLED_PACKAGES_STORE="+filepath.Join(dir, "store"),
		"PACKAGES_PROXY_BOOTSTRAP_ADDRESSES="+proxyAddresses,
		"PACKAGES_PROXY_BOOTSTRAP_CLUSTER_UUID=cluster-uuid",
		"RPP_GET_MARKER="+filepath.Join(dir, "marker"),
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Whoever binds a master's bootstrap port serves what every node runs as root, so nothing from
// that port runs before it is tied to the digest the bundle carries.
func TestRppGetInstallRunsOnlyWhatTheBundleNames(t *testing.T) {
	chdirToRepositoryRoot(t)
	emptyLayer := imageLayer(t, nil)
	genuineLayer := rppGetLayer(t, genuineRppGet)
	substitutedLayer := rppGetLayer(t, substitutedRppGet)
	genuine := rppGetManifest(t, emptyLayer, genuineLayer)
	substituted := rppGetManifest(t, emptyLayer, substitutedLayer)

	for _, tc := range []struct {
		name            string
		bundle          []byte // the manifest whose digest the bundle carries
		manifest, layer []byte // what the proxy serves
		layerHashFails  bool
		refused         string
	}{
		{name: "the genuine proxy", bundle: genuine, manifest: genuine, layer: genuineLayer},
		{name: "a substituted layer behind the genuine manifest", bundle: genuine, manifest: genuine, layer: substitutedLayer, refused: "bytes is not in the manifest"},
		{name: "a substituted manifest", bundle: genuine, manifest: substituted, layer: substitutedLayer, refused: "bytes does not match " + sha256Digest(genuine)},
		{name: "a proxy that serves only the bare binary", bundle: genuine, refused: "manifest download failed"},
		{name: "the empty layer of the genuine image", bundle: genuine, manifest: genuine, layer: emptyLayer, refused: "layer holds no rpp-get"},
		{name: "a substituted layer the node cannot hash", bundle: genuine, manifest: genuine, layer: substitutedLayer, layerHashFails: true, refused: "layer cannot be hashed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			lib := renderRppGetLib(t, sha256Digest(tc.bundle))
			out, err := runRppGetInstall(t, dir, lib, serveRppGet(t, tc.manifest, tc.layer), tc.layerHashFails)

			require.NoFileExists(t, filepath.Join(dir, "marker.substituted"), "%s: the substituted binary ran: %s", tc.name, out)
			if tc.refused != "" {
				require.Error(t, err, out)
				require.Contains(t, out, tc.refused)
				require.NoFileExists(t, filepath.Join(dir, "rpp-get"), out)
				require.NoFileExists(t, filepath.Join(dir, "marker"), "%s: a binary ran: %s", tc.name, out)
				return
			}

			require.NoError(t, err, out)
			require.FileExists(t, filepath.Join(dir, "rpp-get"))
			require.FileExists(t, filepath.Join(dir, "marker"), "the genuine binary answered version")
			recorded, err := os.ReadFile(filepath.Join(dir, "store", "rpp-get", "digest"))
			require.NoError(t, err)
			require.Equal(t, sha256Digest(tc.bundle)+"\n", string(recorded))
		})
	}
}

// A master that serves something else costs the node one refusal, not the install: it goes on to
// the next master.
func TestRppGetInstallMovesOnToTheNextMaster(t *testing.T) {
	chdirToRepositoryRoot(t)
	emptyLayer := imageLayer(t, nil)
	genuineLayer := rppGetLayer(t, genuineRppGet)
	substitutedLayer := rppGetLayer(t, substitutedRppGet)
	genuine := rppGetManifest(t, emptyLayer, genuineLayer)
	substitutedProxy := serveRppGet(t, rppGetManifest(t, emptyLayer, substitutedLayer), substitutedLayer)
	genuineProxy := serveRppGet(t, genuine, genuineLayer)

	dir := t.TempDir()
	out, err := runRppGetInstall(t, dir, renderRppGetLib(t, sha256Digest(genuine)), substitutedProxy+" "+genuineProxy, false)

	require.NoFileExists(t, filepath.Join(dir, "marker.substituted"), "the substituted binary ran: %s", out)
	require.NoError(t, err, out)
	require.Contains(t, out, substitutedProxy+": manifest of")
	require.FileExists(t, filepath.Join(dir, "marker"), out)
}
