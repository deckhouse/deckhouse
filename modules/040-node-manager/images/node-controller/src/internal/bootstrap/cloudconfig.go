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
	writeCloudConfigBody(&out, string(script), in, false)

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
	writeCloudConfigBody(&out, string(script), in, in.Provider == "metal3")

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

const metal3EarlyBootstrapScript = `#!/usr/bin/env bash
set -Eeuo pipefail

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required to discover Metal3 configdrive metadata" >&2
  exit 0
fi

mkdir -p /var/lib/bashible
configdrive_dir=""
cleanup() {
  if [ -n "$configdrive_dir" ]; then
    umount "$configdrive_dir" >/dev/null 2>&1 || true
    rmdir "$configdrive_dir" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

if [ -e /dev/disk/by-label/config-2 ]; then
  configdrive_dir="$(mktemp -d)"
  if mount -o ro /dev/disk/by-label/config-2 "$configdrive_dir" >/dev/null 2>&1; then
    export METAL3_CONFIGDRIVE_METADATA="$configdrive_dir/openstack/latest/meta_data.json"
  fi
fi

python3 - <<'PY'
import json
import os

paths = [
    "/run/cloud-init/instance-data.json",
    "/var/lib/cloud/instance/instance-data.json",
]

data = {}
for path in paths:
    try:
        with open(path, encoding="utf-8") as f:
            data = json.load(f)
        break
    except FileNotFoundError:
        continue
    except json.JSONDecodeError as e:
        print(f"Cannot parse {path}: {e}", file=os.sys.stderr)
        continue

meta = {}
ds = data.get("ds", {})
if isinstance(ds, dict):
    meta.update(ds.get("meta_data") or {})

for key in ("v1", "merged_cfg"):
    value = data.get(key, {})
    if isinstance(value, dict):
        meta.update(value.get("meta_data") or {})

configdrive_metadata = os.environ.get("METAL3_CONFIGDRIVE_METADATA")
if configdrive_metadata:
    try:
        with open(configdrive_metadata, encoding="utf-8") as f:
            meta.update(json.load(f))
    except FileNotFoundError:
        pass
    except json.JSONDecodeError as e:
        print(f"Cannot parse {configdrive_metadata}: {e}", file=os.sys.stderr)

machine_name = meta.get("name") or meta.get("local-hostname") or meta.get("local_hostname")
bmh_name = meta.get("metal3-name")
bmh_namespace = meta.get("metal3-namespace")

if not (machine_name and bmh_name and bmh_namespace):
    print("Metal3 metadata is incomplete; providerID will not be configured", file=os.sys.stderr)
    raise SystemExit(0)

provider_id = f"metal3://{bmh_namespace}/{bmh_name}/{machine_name}"

with open("/var/lib/bashible/machine-name", "w", encoding="utf-8") as f:
    f.write(f"{machine_name}\n")

with open("/var/lib/bashible/node-spec-provider-id", "w", encoding="utf-8") as f:
    f.write(f"{provider_id}\n")
PY
`

// writeCloudConfigBody writes the part both cloud-config flavours share, from
// package_update down to runcmd.
func writeCloudConfigBody(out *bytes.Buffer, script string, in Input, includeMetal3EarlyBootstrap bool) {
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
`, indent4(in.KubernetesCA), in.BootstrapToken)
	if includeMetal3EarlyBootstrap {
		out.WriteString(`- path: /var/lib/bashible/metal3-early-bootstrap.sh
  permissions: '0700'
  content: |
`)
		out.WriteString(indent4(metal3EarlyBootstrapScript))
		out.WriteString("\n")
	}
	out.WriteString(`- path: /var/lib/bashible/first_run
runcmd:
`)
	if includeMetal3EarlyBootstrap {
		out.WriteString("- /var/lib/bashible/metal3-early-bootstrap.sh\n")
	}
	out.WriteString("- /var/lib/bashible/bootstrap.sh\n")
}

// indent4 is sprig's `indent 4`: the pad goes in front of every line, the first
// one included, which is why an argument starting with a newline leaves the pad
// as trailing whitespace on the line before.
func indent4(s string) string {
	const pad = "    "
	return pad + strings.ReplaceAll(s, "\n", "\n"+pad)
}
