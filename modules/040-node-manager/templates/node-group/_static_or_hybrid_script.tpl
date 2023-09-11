{{- define "node_group_static_or_hybrid_script" -}}
  {{- $context := index . 0 -}}
  {{- $ng := index . 1 -}}
  {{- $bootstrap_token := index . 2 -}}
#!/bin/bash

{{ include "node_cleanup" $context }}

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
  {{- $images := set $images "registrypackages" (dict "jq16" $context.Values.global.modulesImages.digests.registrypackages.jq16) }}
  {{- $_ := set $bashible_bootstrap_script_tpl_context "images" $images }}
cat > /var/lib/bashible/bootstrap.sh <<"END"
{{ include "node_group_bashible_bootstrap_script" $bashible_bootstrap_script_tpl_context }}
END
chmod +x /var/lib/bashible/bootstrap.sh

cat > /var/lib/bashible/ca.crt <<"EOF"
{{ $context.Values.nodeManager.internal.kubernetesCA }}
EOF

cat > /var/lib/bashible/bootstrap-token <<"EOF"
{{ $bootstrap_token }}
EOF
chmod 0600 /var/lib/bashible/bootstrap-token

touch /var/lib/bashible/first_run

checkBashible=$(systemctl is-active bashible.timer)
if [[ "$checkBashible" != "active" ]]; then
  /var/lib/bashible/bootstrap.sh
else
  echo "The node already exists in the cluster and under bashible."
fi
{{ end }}

{{- define "node_cleanup" -}}

function node_cleanup() {
  if [ ! -f /etc/kubernetes/kubelet.conf ]; then
    return
  fi
  
  if bb-kubectl --kubeconfig=/etc/kubernetes/kubelet.conf get node "$(hostname -s)" -o json | jq '
    .status.conditions[] | select(.reason=="KubeletReady").status == "True"
  ')"; then
    return
  fi

  while true; do
    msg="The node is not ready. Perhaps the bootstrap failed. Run node cleanup? [yes/no]: "
    read -p "$msg" confirm
    if [ "$confirm" == "yes" ]; then
      break
    elif [ "$confirm" == "no" ]; then
      return
    fi
  done

  systemctl stop kubernetes-api-proxy.service
  systemctl stop kubernetes-api-proxy-configurator.service
  systemctl stop kubernetes-api-proxy-configurator.timer

  systemctl stop bashible.service bashible.timer
  systemctl stop kubelet.service
  systemctl stop containerd

  for i in $(mount -t tmpfs | grep /var/lib/kubelet | cut -d " " -f3); do umount $i ; done

  rm -rf /var/lib/bashible
  rm -rf /var/cache/registrypackages
  rm -rf /etc/kubernetes
  rm -rf /var/lib/kubelet
  rm -rf /var/lib/docker
  rm -rf /var/lib/containerd
  rm -rf /etc/cni
  rm -rf /var/lib/cni
  rm -rf /var/lib/etcd
  rm -rf /etc/systemd/system/kubernetes-api-proxy*
  rm -rf /etc/systemd/system/bashible*
  rm -rf /etc/systemd/system/sysctl-tuner*
  rm -rf /etc/systemd/system/kubelet*
}
node_cleanup
{{- end }}
