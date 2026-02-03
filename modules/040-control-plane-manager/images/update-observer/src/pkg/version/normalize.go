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

package version

import (
	"fmt"
	"strings"

	"golang.org/x/mod/semver"
)

const (
	SemverMajorMinorAccuracy = 2
)

func NormalizeAndTrimPatch(version string) (string, error) {
	if version == "" {
		return "", nil
	}

	normalized := version
	if !strings.HasPrefix(normalized, "v") {
		normalized = "v" + normalized
	}

	if !semver.IsValid(normalized) {
		return "", fmt.Errorf("invalid semver: %s", version)
	}

	parts := strings.Split(normalized, ".")
	if len(parts) < SemverMajorMinorAccuracy {
		return "", fmt.Errorf("version must have at least MAJOR.MINOR: %s", version)
	}

	return fmt.Sprintf("%s.%s", parts[0], parts[1]), nil
}
