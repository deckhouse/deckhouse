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

package proxy

import "strings"

// rewriteRepoPath maps a Docker registry request path from the local repository
// namespace to the upstream one, mirroring distribution's localpathalias /
// remotepathonly. It operates on the repository portion after "/v2/": when the
// repo begins with localAlias, that prefix is replaced by remotePath. Paths that
// are not "/v2/<repo>..." or whose repo does not match localAlias are returned
// unchanged. Leading/trailing slashes in localAlias/remotePath are ignored.
func rewriteRepoPath(path, localAlias, remotePath string) string {
	const v2 = "/v2/"
	if !strings.HasPrefix(path, v2) {
		return path
	}
	rest := path[len(v2):] // "<repo>/<verb>/..."
	if rest == "" {
		return path // "/v2/" ping
	}
	alias := strings.Trim(localAlias, "/")
	remote := strings.Trim(remotePath, "/")

	if alias == "" {
		if remote == "" {
			return path
		}
		return v2 + remote + "/" + rest
	}

	if rest != alias && !strings.HasPrefix(rest, alias+"/") {
		return path // alias not present — leave untouched
	}
	tail := strings.TrimPrefix(strings.TrimPrefix(rest, alias), "/")

	switch {
	case remote == "" && tail == "":
		return v2
	case remote == "":
		return v2 + tail
	case tail == "":
		return v2 + remote
	default:
		return v2 + remote + "/" + tail
	}
}
