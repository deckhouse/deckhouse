{{- define "instance_group_machine_class_cloud_init_cloud_config" }}
  {{- $context := index . 0 }}
  {{- $ng := index . 1 }}
  {{- $zone_name := index . 2 -}}
#cloud-config
package_update: True
manage_etc_hosts: localhost
write_files:

{{- if hasKey $context.Values.cloudInstanceManager.internal "cloudProvider" }}
  {{- range $path, $_ := $context.Files.Glob (printf "cloud-providers/%s/bashible-bundles/*/bootstrap-network.sh" $context.Values.cloudInstanceManager.internal.cloudProvider.type) }}
    {{- $bundle := (dir $path | base) }}
    {{- $tpl_context := dict }}
    {{- $_ := set $tpl_context "Release" $context.Release }}
    {{- $_ := set $tpl_context "Chart" $context.Chart }}
    {{- $_ := set $tpl_context "Files" $context.Files }}
    {{- $_ := set $tpl_context "Capabilities" $context.Capabilities }}
    {{- $_ := set $tpl_context "Template" $context.Template }}
    {{- $_ := set $tpl_context "Values" $context.Values }}
    {{- $_ := set $tpl_context "nodeGroup" $ng }}
- path: '/var/lib/bashible/cloud-provider-bootstrap-network-{{ $bundle }}.sh'
  permissions: '0700'
  encoding: b64
  content: {{ tpl ($context.Files.Get $path) $tpl_context | b64enc }}
  {{- end }}
{{- end }}

- path: '/var/lib/bashible/bootstrap.sh'
  permissions: '0700'
  encoding: b64
  content: {{ include "instance_group_machine_class_bashible_bootstrap_script" $context | b64enc }}

- path: '/var/lib/bashible/bashible.sh'
  permissions: '0700'
  encoding: b64
  content: {{ include "instance_group_machine_class_bashible_bashible_script" (list $context $ng) | b64enc }}

- path: '/var/lib/bashible/ca.crt'
  permissions: '0644'
  encoding: b64
  content: {{ $context.Values.cloudInstanceManager.internal.clusterCA | b64enc }}

- path: /var/lib/bashible/bootstrap-token
  content: <<BOOTSTRAP_TOKEN>>
  permissions: '0600'

runcmd:
- /var/lib/bashible/bootstrap.sh
output:
  all: "| tee -a /var/log/cloud-init-output.log"
{{ end }}
