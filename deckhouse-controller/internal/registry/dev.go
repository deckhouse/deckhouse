// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registry

import (
	"encoding/hex"
	"strings"
)

const (
	// devVersionPrefix marks a version pinned from a dev tag.
	devVersionPrefix = "dev-"
	// sha256Prefix is the algorithm prefix of a sha256 digest.
	sha256Prefix = "sha256:"
	// sha256HexLen is the hex length of a sha256 digest.
	sha256HexLen = 64
)

// DevVersion is the immutable version of a dev image with digest "sha256:<hex>": dev-<hex>.
func DevVersion(digest string) string {
	return devVersionPrefix + strings.TrimPrefix(digest, sha256Prefix)
}

// IsDevVersion reports whether version was made by DevVersion.
func IsDevVersion(version string) bool {
	sum, ok := strings.CutPrefix(version, devVersionPrefix)
	if !ok || len(sum) != sha256HexLen {
		return false
	}

	_, err := hex.DecodeString(sum)

	return err == nil
}

// PullTag is the tag to pull version by: a dev version pulls its digest (the client joins
// repository:dev@sha256:<hex>, a digest reference whose tag is ignored), any other version itself.
func PullTag(version string) string {
	if !IsDevVersion(version) {
		return version
	}

	return "dev@" + sha256Prefix + strings.TrimPrefix(version, devVersionPrefix)
}
