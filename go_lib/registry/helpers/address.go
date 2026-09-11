/*
Copyright 2025 Flant JSC

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

package helpers

import (
	"strings"
)

// SplitAddressAndPath splits a registry reference into its host and its
// repository path.
//
// Both parts are trimmed of surrounding whitespace. Trimming only the reference
// as a whole is not enough: the host is used as a directory name under
// /etc/containerd/registry.d and as a key in the generated hosts.toml, so
// `example.com /path` must not yield the host `example.com `. Whether the host
// is trimmed would otherwise depend on a path following it.
func SplitAddressAndPath(ref string) (string, string) {
	host, path, found := strings.Cut(strings.TrimSpace(strings.TrimRight(ref, "/")), "/")

	host = strings.TrimSpace(host)
	if !found {
		return host, ""
	}

	if path = strings.TrimSpace(path); path == "" {
		return host, ""
	}

	return host, "/" + path
}
