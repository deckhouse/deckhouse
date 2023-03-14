{{- define "node_group_static_or_hybrid_script" -}}
  {{- $context := index . 0 -}}
  {{- $ng := index . 1 -}}
  {{- $bootstrap_token := index . 2 -}}
  {{- $adopt := index . 3 -}}
#!/bin/bash

mkdir -p /var/lib/bashible

  {{- if hasKey $context.Values.nodeManager.internal "cloudProvider" }}
    {{- $bootstrap_scripts_from_bundles := list }}
    {{- range $path, $_ := $context.Files.Glob (printf "candi/cloud-providers/%s/bashible/bundles/*/bootstrap-networks.sh.tpl" $context.Values.nodeManager.internal.cloudProvider.type) }}
      {{- $bootstrap_scripts_from_bundles = append $bootstrap_scripts_from_bundles $path }}
    {{- end }}

    {{- $bootstrap_scripts_common := list }}
    {{- range $path, $_ := $context.Files.Glob (printf "candi/cloud-providers/%s/common-steps/bootstrap-networks.sh.tpl" $context.Values.nodeManager.internal.cloudProvider.type) }}
      {{- $bootstrap_scripts_common = append $bootstrap_scripts_common $path }}
    {{- end }}

    {{- $bootstrap_scripts := list  }}
    {{- if gt (len $bootstrap_scripts_common) 0 }}
      {{- $bootstrap_scripts = $bootstrap_scripts_common }}
    {{- else }}
      {{- $bootstrap_scripts = $bootstrap_scripts_from_bundles }}
    {{- end }}

    {{- range $path := $bootstrap_scripts }}
      {{- $bundle := (dir $path | base) }}
      {{- $tpl_context := dict }}
      {{- $_ := set $tpl_context "Release" $context.Release }}
      {{- $_ := set $tpl_context "Chart" $context.Chart }}
      {{- $_ := set $tpl_context "Files" $context.Files }}
      {{- $_ := set $tpl_context "Capabilities" $context.Capabilities }}
      {{- $_ := set $tpl_context "Template" $context.Template }}
      {{- $_ := set $tpl_context "Values" $context.Values }}
      {{- $_ := set $tpl_context "nodeGroup" $ng }}
cat > /var/lib/bashible/cloud-provider-bootstrap-networks-{{ $bundle }}.sh <<"EOF"
{{ tpl ($context.Files.Get $path) $tpl_context }}
EOF
chmod +x /var/lib/bashible/cloud-provider-bootstrap-networks-{{ $bundle }}.sh
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
cat > /var/lib/bashible/bootstrap.sh <<"END"
{{ if $adopt }}
  {{- include "node_group_bashible_bootstrap_script_noninteractive" $bashible_bootstrap_script_tpl_context }}
{{ else }}
  {{- include "node_group_bashible_bootstrap_script" $bashible_bootstrap_script_tpl_context }}
{{ end }}
END
chmod +x /var/lib/bashible/bootstrap.sh

cat > /var/lib/bashible/ca.crt <<"EOF"
{{ $context.Values.nodeManager.internal.kubernetesCA }}
EOF

cat > /var/lib/bashible/bootstrap-token <<"EOF"
{{ $bootstrap_token }}
EOF
chmod 0600 /var/lib/bashible/bootstrap-token

{{- if not $adopt }}
touch /var/lib/bashible/first_run
{{- end }}

checkBashible=$(systemctl is-active bashible.timer)
if [[ "$checkBashible" != "active" ]]; then
  /var/lib/bashible/bootstrap.sh
else
  echo "The node already exists in the cluster and under bashible."
fi
{{ end }}
