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

package providerdir

import (
	"os"
	"path/filepath"
	"strings"
)

// ProviderDir returns the stable per-provider root under root
// (<root>/<provider>). Once a bundle is unpacked it is a symlink to the
// current ProviderDigestDir.
func ProviderDir(root, provider string) string {
	return filepath.Join(root, strings.ToLower(provider))
}

// ProviderDigestDir returns the digest-pinned unpack directory for a provider
// bundle (<root>/<provider>@<digest>).
func ProviderDigestDir(root, provider, digest string) string {
	return filepath.Join(root, strings.ToLower(provider)+"@"+digest)
}

// ValidatorPath returns the expected location of the provider's external
// validator binary inside the unpacked bundle.
func ValidatorPath(root, provider string) string {
	return filepath.Join(ProviderDir(root, provider), "validator")
}

// SchemaPath returns the ClusterConfiguration schema inside an unpacked bundle.
// A bundle without it carries nothing dhctl can validate against.
func SchemaPath(dir string) string {
	return filepath.Join(dir, "openapi", "cluster_configuration.yaml")
}

// DigestFromDir reads the digest out of a ProviderDigestDir name. Empty for any
// other directory.
func DigestFromDir(dir string) string {
	_, digest, found := strings.Cut(filepath.Base(dir), "@")
	if !found {
		return ""
	}
	return digest
}

// Delivered resolves the provider's default alias and reports the digest-pinned
// directory it currently points at, together with that digest. Not delivered
// when the alias is missing, is not a symlink, dangles, or leads anywhere but a
// digest dir directly under root.
func Delivered(root, provider string) (string, string, bool) {
	link := ProviderDir(root, provider)
	info, err := os.Lstat(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return "", "", false
	}

	dir, err := filepath.EvalSymlinks(link)
	if err != nil {
		return "", "", false
	}

	// The alias and the digest dir are siblings, so the comparison is against
	// the resolved root: a symlinked DownloadDir would otherwise never match.
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil || filepath.Dir(dir) != resolvedRoot {
		return "", "", false
	}

	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return "", "", false
	}

	digest, ok := strings.CutPrefix(filepath.Base(dir), strings.ToLower(provider)+"@")
	if !ok || digest == "" {
		return "", "", false
	}
	return dir, digest, true
}
