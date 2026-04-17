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

var partialPathSegmentRegexp = regexp.MustCompile(`^[a-z0-9-]+$`)
var partialMarkdownFileRegexp = regexp.MustCompile(`^[a-z0-9-]+(?:-v[0-9]+)?(?:\.ru)?\.md$`)

func isPartialStaticPath(path string) bool {
	return strings.HasPrefix(path, "/partials/static/")
}

func validatePartialPath(path string) error {
	if !strings.HasPrefix(path, "/partials/") {
		return nil
	}

	if strings.Contains(strings.TrimPrefix(path, "/partials/"), "/static/") && !strings.HasPrefix(path, "/partials/static/") {
		return fmt.Errorf("nested static directory is not allowed in partials path: %s", path)
	}

	if isPartialStaticPath(path) {
		return nil
	}

	if !strings.HasSuffix(path, ".md") || strings.HasSuffix(path, "/") {
		return fmt.Errorf("unsupported partial artifact path: %s", path)
	}

	segments := strings.Split(strings.TrimPrefix(path, "/partials/"), "/")
	for i, segment := range segments {
		if i == len(segments)-1 {
			if !partialMarkdownFileRegexp.MatchString(segment) {
				return fmt.Errorf("invalid partial file name: %s", segment)
			}
			continue
		}

		if segment == "static" || !partialPathSegmentRegexp.MatchString(segment) {
			return fmt.Errorf("invalid partial path segment: %s", segment)
		}
	}

	return nil
}

func partialStaticOutputPath(baseDir, moduleName, channel string) string {
	return filepath.Join(baseDir, partialsDir, moduleName, channel, "partials", "static")
}
