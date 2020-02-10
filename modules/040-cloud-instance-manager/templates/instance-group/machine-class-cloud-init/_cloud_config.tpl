{{- define "instance_group_machine_class_cloud_init_cloud_config" }}
  {{- $context := index . 0 }}
  {{- $ig := index . 1 }}
  {{- $zone_name := index . 2 -}}
#cloud-config
package_update: True
write_files:
- path: '/var/lib/bashible/ca.crt'
  permissions: '0644'
  encoding: b64
  content: {{ $context.Values.cloudInstanceManager.internal.clusterCA | b64enc }}
{{- $bashible_bundle := $ig.instanceClass.bashible.bundle }}
{{- if $file := $context.Files.Get (list "cloud-providers" $context.Values.cloudInstanceManager.internal.cloudProvider.type "bashible-bundles" $bashible_bundle "bootstrap.sh" | join "/") }}
  {{- $tpl_context := dict }}
  {{- $_ := set $tpl_context "Release" $context.Release }}
  {{- $_ := set $tpl_context "Chart" $context.Chart }}
  {{- $_ := set $tpl_context "Files" $context.Files }}
  {{- $_ := set $tpl_context "Capabilities" $context.Capabilities }}
  {{- $_ := set $tpl_context "Template" $context.Template }}
  {{- $_ := set $tpl_context "Values" $context.Values }}
  {{- $_ := set $tpl_context "instanceGroup" $ig }}
  {{- $_ := set $tpl_context "zoneName" $zone_name }}
- path: '/var/lib/bashible/cloud-provider-bootstrap-{{ $bashible_bundle }}.sh'
  permissions: '0700'
  encoding: b64
  content: {{ tpl $file $tpl_context | b64enc }}
{{- end }}
- path: '/var/lib/bashible/bashible.sh'
  permissions: '0700'
  encoding: b64
  content: {{ include "instance_group_machine_class_bashible_bashible_script" (list $context $ig $zone_name) | b64enc }}
- path: '/var/lib/bashible/bootstrap.sh'
  permissions: '0700'
  encoding: b64
  content: {{ include "instance_group_machine_class_bashible_bootstrap_script" (list $context $ig $zone_name) | b64enc }}
output: { all: "| tee -a /var/log/cloud-init-output.log" }

runcmd:
- /var/lib/bashible/bootstrap.sh
{{ end }}
