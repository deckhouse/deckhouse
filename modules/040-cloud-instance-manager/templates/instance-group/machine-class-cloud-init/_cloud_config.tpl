{{- define "instance_group_machine_class_cloud_init_cloud_config" }}
  {{- $context := index . 0 }}
  {{- $ig := index . 1 }}
  {{- $zone_name := index . 2 -}}
#cloud-config
package_update: True
packages: ['jq']
write_files:
- path: '/var/lib/machine-bootstrap/ca.crt'
  permissions: '0644'
  encoding: b64
  content: {{ $context.Values.cloudInstanceManager.internal.clusterCA | b64enc }}
{{- $cloud_init_steps_version := $ig.instanceClass.cloudInitSteps.version | default $context.Values.cloudInstanceManager.internal.cloudInitSteps.version }}
{{- if $file := $context.Files.Get (list "cloud-providers" $context.Values.cloudInstanceManager.internal.cloudProvider.type "cloud-init-steps" $cloud_init_steps_version "bootstrap.sh" | join "/") }}
  {{- $tpl_context := dict }}
  {{- $_ := set $tpl_context "Release" $context.Release }}
  {{- $_ := set $tpl_context "Chart" $context.Chart }}
  {{- $_ := set $tpl_context "Files" $context.Files }}
  {{- $_ := set $tpl_context "Capabilities" $context.Capabilities }}
  {{- $_ := set $tpl_context "Template" $context.Template }}
  {{- $_ := set $tpl_context "Values" $context.Values }}
  {{- $_ := set $tpl_context "instanceGroup" $ig }}
  {{- $_ := set $tpl_context "zoneName" $zone_name }}
- path: '/var/lib/machine-bootstrap/cloud-provider-bootstrap-{{ $cloud_init_steps_version }}.sh'
  permissions: '0700'
  encoding: b64
  content: {{ tpl $file $tpl_context | b64enc }}
{{- end }}
- path: '/var/lib/machine-bootstrap/bootstrap.sh'
  permissions: '0700'
  encoding: b64
  content: {{ include "instance_group_machine_class_cloud_init_bootstrap_script" (list $context $ig $zone_name) | b64enc }}

runcmd:
- /var/lib/machine-bootstrap/bootstrap.sh
{{ end }}
