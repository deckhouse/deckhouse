{{- define "node_group_cloud_init_cloud_config" }}
  {{- $context := index . 0 }}
  {{- $ng := index . 1 }}
  {{- $bootstrap_token := index . 2 -}}
#cloud-config
  {{- if ($context.Values.global.enabledModules | has "cloud-provider-azure") }}
mounts:
- [ ephemeral0, /mnt/resource ]
  {{- end }}
package_update: True
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
#cloud-config

ssh_authorized_keys:
- {{ $context.Values.nodeManager.internal.cloudProvider.sshPublicKey| default "" | quote }}
package_update: True
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
