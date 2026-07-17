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

package digests

import (
	"fmt"
	"os"
)

// ImagesDigestsFileEnv points dhctl at an images_digests.json on disk instead
// of the compile-time embedded copy. In-cluster dhctl (terraform-auto-converger,
// terraform-state-exporter) cannot embed real digests — that would create a
// werf build cycle (their images are themselves listed in images-digests) — so
// the terraform-manager module mounts the cluster's digests via a ConfigMap
// and sets this variable.
const ImagesDigestsFileEnv = "DHCTL_IMAGES_DIGESTS_FILE"

// fileDigestsContent reads digests from the file named by ImagesDigestsFileEnv.
// ok is false when the variable is unset (caller falls back to embedded).
// A set-but-unreadable file is an error, not a fallback: the environment
// explicitly promised a file, silently using stale embedded digests instead
// would reintroduce the bug this mechanism fixes.
func fileDigestsContent() ([]byte, bool, error) {
	path := os.Getenv(ImagesDigestsFileEnv)
	if path == "" {
		return nil, false, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, true, fmt.Errorf("read images digests from %s (%s): %w", path, ImagesDigestsFileEnv, err)
	}
	return content, true, nil
}
