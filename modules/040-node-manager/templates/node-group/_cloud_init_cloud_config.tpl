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

{{- if hasKey $context.Values.nodeManager.internal "cloudProvider" }}
  {{- $tpl_context := dict }}
  {{- $_ := set $tpl_context "Release" $context.Release }}
  {{- $_ := set $tpl_context "Chart" $context.Chart }}
  {{- $_ := set $tpl_context "Files" $context.Files }}
  {{- $_ := set $tpl_context "Capabilities" $context.Capabilities }}
  {{- $_ := set $tpl_context "Template" $context.Template }}
  {{- $_ := set $tpl_context "Values" $context.Values }}
  {{- $_ := set $tpl_context "nodeGroup" $ng }}

  {{- if $bootstrap_script_common := $context.Files.Get (printf "candi/cloud-providers/%s/bashible/common-steps/bootstrap-networks.sh.tpl" $context.Values.nodeManager.internal.cloudProvider.type)  }}
- path: '/var/lib/bashible/cloud-provider-bootstrap-networks.sh'
  permissions: '0700'
  content: |
    {{- tpl $bootstrap_script_common $tpl_context | nindent 4 }}
  {{- else }}
    {{- range $path, $_ := $context.Files.Glob (printf "candi/cloud-providers/%s/bashible/bundles/*/bootstrap-networks.sh.tpl" $context.Values.nodeManager.internal.cloudProvider.type) }}
      {{- $bundle := (dir $path | base) }}
- path: '/var/lib/bashible/cloud-provider-bootstrap-networks-{{ $bundle }}.sh'
  permissions: '0700'
  content: |
    {{- tpl ($context.Files.Get $path) $tpl_context | nindent 4 }}
    {{- end }}
  {{- end }}
{{- end }}

{{- $bashible_bootstrap_script_tpl_context := dict }}
{{- $_ := set $bashible_bootstrap_script_tpl_context "normal" (dict "apiserverEndpoints" $context.Values.nodeManager.internal.clusterMasterAddresses) }}
{{- $_ := set $bashible_bootstrap_script_tpl_context "nodeGroup" $ng }}
{{- $_ := set $bashible_bootstrap_script_tpl_context "Template" $context.Template }}
{{- $_ := set $bashible_bootstrap_script_tpl_context "Files" $context.Files }}
{{- $_ := set $bashible_bootstrap_script_tpl_context "allowedBundles" $context.Values.nodeManager.allowedBundles }}
{{- $_ := set $bashible_bootstrap_script_tpl_context "registry" (dict "address" $context.Values.global.modulesImages.registry.address "path" $context.Values.global.modulesImages.registry.path "scheme" $context.Values.global.modulesImages.registry.scheme "ca" $context.Values.global.modulesImages.registry.CA "dockerCfg" $context.Values.global.modulesImages.registry.dockercfg) }}
{{- if hasKey $context.Values.global.clusterConfiguration "proxy" }}
  {{- $proxy := dict }}
  {{- if hasKey $context.Values.global.clusterConfiguration.proxy "httpProxy" }}
    {{- $_ := set $proxy "httpProxy" $context.Values.global.clusterConfiguration.proxy.httpProxy }}
  {{- end }}
  {{- if hasKey $context.Values.global.clusterConfiguration.proxy "httpsProxy" }}
    {{- $_ := set $proxy "httpsProxy" $context.Values.global.clusterConfiguration.proxy.httpsProxy }}
  {{- end }}
  {{- $noProxy := list "127.0.0.1" "169.254.169.254" $context.Values.global.clusterConfiguration.clusterDomain $context.Values.global.clusterConfiguration.podSubnetCIDR $context.Values.global.clusterConfiguration.serviceSubnetCIDR }}
  {{- if hasKey $context.Values.global.clusterConfiguration.proxy "noProxy" }}
    {{- $noProxy = concat $noProxy $context.Values.global.clusterConfiguration.proxy.noProxy }}
  {{- end }}
  {{- $_ := set $proxy "noProxy" $noProxy }}
  {{- $_ := set $bashible_bootstrap_script_tpl_context "proxy" $proxy }}
{{- end }}
{{- /* For centos bootstrap script jq package tag is needed */ -}}
{{- $images := dict }}
{{- $images := set $images "registrypackages" (dict "jq16" $context.Values.global.modulesImages.tags.registrypackages.jq16) }}
{{- $_ := set $bashible_bootstrap_script_tpl_context "images" $images }}

- path: '/var/lib/bashible/bootstrap.sh'
  permissions: '0700'
  content: |
    {{- include "node_group_bashible_bootstrap_script" $bashible_bootstrap_script_tpl_context | nindent 4 }}

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
output:
  all: "| tee -a /var/log/cloud-init-output.log"
{{ end }}
