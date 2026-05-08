{{- define "bootstrap_script" }}
  {{- $context := index . 0 }}
  {{- $ng := index . 1 }}
  {{- $tpl_context := dict }}
  {{- $_ := set $tpl_context "Release" $context.Release }}
  {{- $_ := set $tpl_context "Chart" $context.Chart }}
  {{- $_ := set $tpl_context "Files" $context.Files }}
  {{- $_ := set $tpl_context "Capabilities" $context.Capabilities }}
  {{- $_ := set $tpl_context "Template" $context.Template }}
  {{- $_ := set $tpl_context "Values" $context.Values }}
  {{- $_ := set $tpl_context "nodeGroup" $ng }}
  {{- $_ := set $tpl_context "apiserverEndpoints" $context.Values.nodeManager.internal.clusterMasterAddresses }}
  {{- $clusterMasterEndpoints := $context.Values.nodeManager.internal.clusterMasterEndpoints | default (list) }}
  {{- $clusterMasterKubeAPIEndpoints := list }}
  {{- $clusterMasterRPPAddresses := list }}
  {{- $clusterMasterRPPBootstrapAddresses := list }}
  {{- range $endpoint := $clusterMasterEndpoints }}
    {{- if hasKey $endpoint "kubeApiPort" }}
      {{- $clusterMasterKubeAPIEndpoints = append $clusterMasterKubeAPIEndpoints (printf "%s:%v" $endpoint.address $endpoint.kubeApiPort) }}
    {{- end }}
    {{- if hasKey $endpoint "rppServerPort" }}
      {{- $clusterMasterRPPAddresses = append $clusterMasterRPPAddresses (printf "%s:%v" $endpoint.address $endpoint.rppServerPort) }}
    {{- end }}
    {{- if hasKey $endpoint "rppBootstrapServerPort" }}
      {{- $clusterMasterRPPBootstrapAddresses = append $clusterMasterRPPBootstrapAddresses (printf "%s:%v" $endpoint.address $endpoint.rppBootstrapServerPort) }}
    {{- end }}
  {{- end }}
  {{- $_ := set $tpl_context "clusterMasterEndpoints" $clusterMasterEndpoints }}
  {{- $_ := set $tpl_context "clusterMasterKubeAPIEndpoints" $clusterMasterKubeAPIEndpoints }}
  {{- $_ := set $tpl_context "clusterMasterRPPAddresses" $clusterMasterRPPAddresses }}
  {{- $_ := set $tpl_context "clusterMasterRPPBootstrapAddresses" $clusterMasterRPPBootstrapAddresses }}
  {{- $_ := set $tpl_context "clusterUUID" ($context.Values.global.discovery.clusterUUID | default "") }}
  {{- $_ := set $tpl_context "images" $context.Values.global.modulesImages.digests }}
  {{- $packagesProxy := $context.Values.nodeManager.internal.packagesProxy | default (dict) }}
  {{- $_ := set $tpl_context "packagesProxy" $packagesProxy }}
  {{- $_ := set $tpl_context "mingetB64" ($context.Files.Get "candi/bashible/bootstrap/minget" | b64enc) }}
  {{- if hasKey $context.Values.nodeManager.internal "cloudProvider" }}
    {{- $_ := set $tpl_context "provider" $context.Values.nodeManager.internal.cloudProvider.type }}
  {{- end }}
#!/usr/bin/env bash
set -Eeuo pipefail
BOOTSTRAP_DIR="/var/lib/bashible"
TMPDIR="/opt/deckhouse/tmp"
mkdir -p "${BOOTSTRAP_DIR}" "${TMPDIR}"
export PATH="/opt/deckhouse/bin:/usr/local/bin:$PATH"
bootstrap_log_init() {
  if [[ -z ${BOOTSTRAP_LOG:-} ]]; then
    mkdir -p /var/log/d8/bashible
    exec {stdout_fd}>&1
    exec > >(tee -a /var/log/d8/bashible/bootstrap.log >&${stdout_fd}) 2>&1
    export BOOTSTRAP_LOG=1
  fi
}
bootstrap_log_init
{{- $candi := "candi/bashible/lib.sh.tpl" -}}
{{- $deckhouse := "/deckhouse/candi/bashible/lib.sh.tpl" -}}
{{- $lib := $context.Files.Get $deckhouse | default ($context.Files.Get $candi) -}}
{{- $ctx := $tpl_context -}}
{{- tpl (printf `%s
{{ template "get-phase2" $ }}` $lib) $ctx }}
{{- if $fetch_base_pkgs := $context.Files.Get "candi/bashible/bootstrap/01-bootstrap-prerequisites.sh.tpl" }}
  {{- $fetch_base_pkgs = regexReplaceAll "^#!/bin/bash\nset -Eeo pipefail\n" $fetch_base_pkgs "" }}
  {{- tpl ( $fetch_base_pkgs ) $tpl_context | nindent 0 }}
{{- end }}
{{- if or (eq $ng.nodeType "CloudEphemeral") (hasKey $ng "staticInstances") }}
/opt/deckhouse/bin/tail-log ${TMPDIR}/bootstrap.log &
bootstrap_job_log_pid=$!
{{- end }}
get_phase2 | bash
if [ -n "${bootstrap_job_log_pid:-}" ]; then
  kill -9 "${bootstrap_job_log_pid}"
fi

{{- end }}
