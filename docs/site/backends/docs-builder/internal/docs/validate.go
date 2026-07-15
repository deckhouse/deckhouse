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
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// nameRegexp restricts module and channel names to a strict allow-list.
//
// It mirrors the RFC 1123 label naming used by Kubernetes object names (where
// legitimate module names originate) and, crucially, forbids every character
// usable for path traversal — a matching value contains no '/', '.' or '..'.
// This is the primary guard against CWE-22: moduleName and channel are
// attacker-controllable request parameters that get joined into filesystem
// paths (see cleanModulesFiles and getLocalPath).
var nameRegexp = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// validateModuleName rejects a module name that could escape the base
// directory when joined into a filesystem path.
func validateModuleName(moduleName string) error {
	if !nameRegexp.MatchString(moduleName) {
		return fmt.Errorf("invalid module name %q: must match %s", moduleName, nameRegexp.String())
	}

	return nil
}

// validateChannels rejects an empty channel list or any channel name that
// could escape the base directory when joined into a filesystem path.
func validateChannels(channels []string) error {
	if len(channels) == 0 {
		return fmt.Errorf("no channels provided")
	}

	for _, channel := range channels {
		if !nameRegexp.MatchString(channel) {
			return fmt.Errorf("invalid channel %q: must match %s", channel, nameRegexp.String())
		}
	}

	return nil
}

// ensureWithinBase reports an error if the cleaned path escapes baseDir. It is
// a defense-in-depth complement to validateModuleName/validateChannels: even if
// some new path segment were to slip past name validation, a write or delete
// outside baseDir is still refused.
func ensureWithinBase(baseDir, path string) error {
	rel, err := filepath.Rel(baseDir, path)
	if err != nil {
		return fmt.Errorf("resolve %q against %q: %w", path, baseDir, err)
	}

	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q escapes base directory %q", path, baseDir)
	}

	return nil
}
