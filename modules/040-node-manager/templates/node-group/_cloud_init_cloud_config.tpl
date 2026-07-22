{{- define "node_group_cloud_init_cloud_config" }}
  {{- $context := index . 0 }}
  {{- $ng := index . 1 }}
  {{- $bootstrap_token := index . 2 -}}
#cloud-config
  {{- if and (hasKey $context.Values.nodeManager.internal "cloudProvider") (eq $context.Values.nodeManager.internal.cloudProvider.type "azure") }}
mounts:
- [ ephemeral0, /mnt/resource ]
  {{- end }}
package_update: false
package_upgrade: false
manage_etc_hosts: localhost
write_files:
- path: '/var/lib/bashible/bootstrap.sh'
  permissions: '0700'
  content: |
    {{- include "bootstrap_script" (list $context $ng) | indent 4 }}
- path: '/var/lib/bashible/ca.crt'
  permissions: '0644'
  content: |
    {{- $context.Values.nodeManager.internal.kubernetesCA | nindent 4 }}
- path: /var/lib/bashible/bootstrap-token
  content: {{ $bootstrap_token }}
  permissions: '0600'
- path: /var/lib/bashible/first_run
runcmd:
- /var/lib/bashible/bootstrap.sh
{{ end }}

{{- define "node_group_capi_cloud_init_cloud_config" }}
  {{- $context := index . 0 }}
  {{- $ng := index . 1 }}
  {{- $bootstrap_token := index . 2 -}}
  {{- $provider := $context.Values.nodeManager.internal.cloudProvider.type | default "" -}}
#cloud-config

ssh_authorized_keys:
- {{ $context.Values.nodeManager.internal.cloudProvider.sshPublicKey| default "" | quote }}
package_update: false
package_upgrade: false
manage_etc_hosts: localhost
write_files:
- path: '/var/lib/bashible/bootstrap.sh'
  permissions: '0700'
  content: |
    {{- include "bootstrap_script" (list $context $ng) | indent 4 }}
- path: '/var/lib/bashible/ca.crt'
  permissions: '0644'
  content: |
    {{- $context.Values.nodeManager.internal.kubernetesCA | nindent 4 }}
- path: /var/lib/bashible/bootstrap-token
  content: {{ $bootstrap_token }}
  permissions: '0600'
- path: /var/lib/bashible/first_run
{{- if eq $provider "metal3" }}
- path: /var/lib/bashible/metal3-early-bootstrap.sh
  permissions: '0700'
  content: |
    #!/usr/bin/env bash
    set -Eeuo pipefail

    if ! command -v python3 >/dev/null 2>&1; then
      echo "python3 is required to discover Metal3 configdrive metadata" >&2
      exit 0
    fi

    mkdir -p /var/lib/bashible
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
{{- end }}
runcmd:
{{- if eq $provider "metal3" }}
- /var/lib/bashible/metal3-early-bootstrap.sh
{{- end }}
- /var/lib/bashible/bootstrap.sh
{{ end }}
