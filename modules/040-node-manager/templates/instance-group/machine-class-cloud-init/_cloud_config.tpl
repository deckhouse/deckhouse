{{- define "node_group_machine_class_cloud_init_cloud_config" }}
  {{- $context := index . 0 }}
  {{- $ng := index . 1 }}
#cloud-config
package_update: True
manage_etc_hosts: localhost
write_files:

{{- if hasKey $context.Values.nodeManager.internal "cloudProvider" }}
  {{- range $path, $_ := $context.Files.Glob (printf "candi/cloud-providers/%s/bashible-bundles/*/bootstrap-networks.sh.tpl" $context.Values.nodeManager.internal.cloudProvider.type) }}
    {{- $bundle := (dir $path | base) }}
    {{- $tpl_context := dict }}
    {{- $_ := set $tpl_context "Release" $context.Release }}
    {{- $_ := set $tpl_context "Chart" $context.Chart }}
    {{- $_ := set $tpl_context "Files" $context.Files }}
    {{- $_ := set $tpl_context "Capabilities" $context.Capabilities }}
    {{- $_ := set $tpl_context "Template" $context.Template }}
    {{- $_ := set $tpl_context "Values" $context.Values }}
    {{- $_ := set $tpl_context "nodeGroup" $ng }}
- path: '/var/lib/bashible/cloud-provider-bootstrap-networks-{{ $bundle }}.sh'
  permissions: '0700'
  encoding: b64
  content: {{ tpl ($context.Files.Get $path) $tpl_context | b64enc }}
  {{- end }}
{{- end }}

{{- $bashible_bootstrap_script_tpl_context := dict }}
{{- $_ := set $bashible_bootstrap_script_tpl_context "normal" (dict "apiserverEndpoints" $context.Values.nodeManager.internal.clusterMasterAddresses) }}
{{- $_ := set $bashible_bootstrap_script_tpl_context "nodeGroup" $ng }}
{{- $_ := set $bashible_bootstrap_script_tpl_context "Template" $context.Template }}
{{- $_ := set $bashible_bootstrap_script_tpl_context "Files" $context.Files }}
- path: '/var/lib/bashible/bootstrap.sh'
  permissions: '0700'
  encoding: b64
  content: {{ include "node_group_machine_class_bashible_bootstrap_script" $bashible_bootstrap_script_tpl_context | b64enc }}

- path: '/var/lib/bashible/ca.crt'
  permissions: '0644'
  encoding: b64
  content: {{ $context.Values.nodeManager.internal.kubernetesCA | b64enc }}

- path: /var/lib/bashible/bootstrap-token
  content: <<BOOTSTRAP_TOKEN>>
  permissions: '0600'

runcmd:
- /var/lib/bashible/bootstrap.sh
output:
  all: "| tee -a /var/log/cloud-init-output.log"
{{ end }}
