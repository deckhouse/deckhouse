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

package bootstrap

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// The three payloads below are assembled as text, not marshalled from structs:
// helm emits them through indent/nindent, and its output is not what a YAML
// marshaller produces (the block scalar carries the pad as trailing spaces, the
// azure mount is a flow sequence). A node reads these bytes, so they must be
// helm's bytes.

// azureMounts is helm's ephemeral-disk mount, emitted for azure only.
const azureMounts = `mounts:
- [ ephemeral0, /mnt/resource ]
`

// RenderCloudConfig renders the cloud-init a bashible node boots from: the
// cloud-config key of the manual bootstrap secret and the userData of an MCM
// machine-class secret. Port of the helm define
// "node_group_cloud_init_cloud_config" (_cloud_init_cloud_config.tpl).
func RenderCloudConfig(in Input) ([]byte, error) {
	script, err := RenderScript(in)
	if err != nil {
		return nil, fmt.Errorf("render bootstrap script: %w", err)
	}

	var out bytes.Buffer
	out.WriteString("#cloud-config\n")
	if in.Provider == "azure" {
		out.WriteString(azureMounts)
	}
	writeCloudConfigBody(&out, string(script), in)

	return out.Bytes(), nil
}

// RenderCAPICloudConfig renders the value key of a CAPI bootstrap secret. Port
// of the helm define "node_group_capi_cloud_init_cloud_config"
// (_cloud_init_cloud_config.tpl).
func RenderCAPICloudConfig(in Input) ([]byte, error) {
	script, err := RenderScript(in)
	if err != nil {
		return nil, fmt.Errorf("render bootstrap script: %w", err)
	}

	var out bytes.Buffer
	out.WriteString("#cloud-config\n\nssh_authorized_keys:\n- ")
	out.WriteString(strconv.Quote(in.SSHPublicKey))
	out.WriteString("\n")
	writeCloudConfigBody(&out, string(script), in)

	return out.Bytes(), nil
}

// RenderStaticScript renders the bootstrap.sh key of the manual bootstrap
// secret: the script an operator runs on a static node. Port of the helm define
// "node_group_static_or_hybrid_script" (_static_or_hybrid_script.tpl).
func RenderStaticScript(in Input) ([]byte, error) {
	script, err := RenderScript(in)
	if err != nil {
		return nil, fmt.Errorf("render bootstrap script: %w", err)
	}

	var out bytes.Buffer
	out.WriteString(`#!/bin/bash

if [[ -f /var/lib/bashible/bootstrap-token ]]; then
  echo "The node already have bootstrap-token and under bashible."
  exit 1
fi

checkBashible=$(systemctl is-active bashible.timer)
if [[ "$checkBashible" == "active" ]]; then
  echo "The node already exists in the cluster and under bashible."
  exit 2
fi

mkdir -p /var/lib/bashible

cat > /var/lib/bashible/bootstrap.sh <<"END"`)
	out.Write(script)
	fmt.Fprintf(&out, `
END
chmod +x /var/lib/bashible/bootstrap.sh

cat > /var/lib/bashible/ca.crt <<"EOF"
%s
EOF

cat > /var/lib/bashible/bootstrap-token <<"EOF"
%s
EOF
chmod 0600 /var/lib/bashible/bootstrap-token

touch /var/lib/bashible/first_run

/var/lib/bashible/bootstrap.sh
`, in.KubernetesCA, in.BootstrapToken)

	return out.Bytes(), nil
}

// writeCloudConfigBody writes the part both cloud-config flavours share, from
// package_update down to runcmd.
func writeCloudConfigBody(out *bytes.Buffer, script string, in Input) {
	out.WriteString(`package_update: false
package_upgrade: false
manage_etc_hosts: localhost
write_files:
- path: '/var/lib/bashible/bootstrap.sh'
  permissions: '0700'
  content: |`)
	out.WriteString(indent4(script))
	fmt.Fprintf(out, `
- path: '/var/lib/bashible/ca.crt'
  permissions: '0644'
  content: |
%s
- path: /var/lib/bashible/bootstrap-token
  content: %s
  permissions: '0600'
- path: /var/lib/bashible/first_run
runcmd:
- /var/lib/bashible/bootstrap.sh
`, indent4(in.KubernetesCA), in.BootstrapToken)
}

// indent4 is sprig's `indent 4`: the pad goes in front of every line, the first
// one included, which is why an argument starting with a newline leaves the pad
// as trailing whitespace on the line before.
func indent4(s string) string {
	const pad = "    "
	return pad + strings.ReplaceAll(s, "\n", "\n"+pad)
}
