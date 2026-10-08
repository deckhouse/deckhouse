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
	"path/filepath"
	"strings"
	"time"
)

// StagingDirMaxAge is the age at which a <digest-dir>.partial-N staging dir counts as left by a
// process that is gone: an unpack takes minutes.
const StagingDirMaxAge = time.Hour

// DigestDir returns the directory an image is unpacked into
// (<root>/<name>@<digest>). The tmp cleaner keeps every directory of this shape.
func DigestDir(root, name, digest string) string {
	return filepath.Join(root, strings.ToLower(name)+"@"+digest)
}

// ValidatorPath returns the provider's external validator binary inside an unpacked bundle.
func ValidatorPath(bundleDir string) string {
	return filepath.Join(bundleDir, "validator")
}

// SchemaPath returns the ClusterConfiguration schema inside an unpacked bundle.
// A bundle without it carries nothing dhctl can validate against.
func SchemaPath(dir string) string {
	return filepath.Join(dir, "openapi", "cluster_configuration.yaml")
}

// DigestFromDir reads the digest out of a DigestDir name. Empty for any
// other directory.
func DigestFromDir(dir string) string {
	_, digest, found := strings.Cut(filepath.Base(dir), "@")
	if !found {
		return ""
	}
	return digest
}
