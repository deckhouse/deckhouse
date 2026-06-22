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

package containerd

import (
	"fmt"
	"strings"
)

// renderHostsTOML renders a containerd hosts.toml for one managed host.
// caPaths[i] is the on-disk path of the CA file for host.Entries[i], or ""
// when that entry has no CA. Entries are emitted in order; containerd tries
// them top to bottom, so the first entry is the preferred route.
func renderHostsTOML(host HostConfig, caPaths []string) (string, error) {
	if len(caPaths) != len(host.Entries) {
		return "", fmt.Errorf("caPaths length %d does not match entries length %d", len(caPaths), len(host.Entries))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "server = %q\n", host.Server)

	for i, e := range host.Entries {
		b.WriteByte('\n')
		fmt.Fprintf(&b, "[host.%q]\n", e.URL)

		quoted := make([]string, len(e.Capabilities))
		for j, c := range e.Capabilities {
			quoted[j] = fmt.Sprintf("%q", c)
		}
		fmt.Fprintf(&b, "  capabilities = [%s]\n", strings.Join(quoted, ", "))

		if caPaths[i] != "" {
			fmt.Fprintf(&b, "  ca = [%q]\n", caPaths[i])
		}
		if e.SkipVerify {
			b.WriteString("  skip_verify = true\n")
		}
		if e.Auth != "" {
			fmt.Fprintf(&b, "  [host.%q.auth]\n", e.URL)
			fmt.Fprintf(&b, "    auth = %q\n", e.Auth)
		}
	}

	return b.String(), nil
}
