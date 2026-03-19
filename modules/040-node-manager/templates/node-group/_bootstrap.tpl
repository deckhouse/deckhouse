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
  {{- $_ := set $tpl_context "clusterMasterEndpoints" ($context.Values.nodeManager.internal.clusterMasterEndpoints | default (list)) }}
  {{- $_ := set $tpl_context "clusterUUID" ($context.Values.global.discovery.clusterUUID | default "") }}
  {{- $_ := set $tpl_context "images" $context.Values.global.modulesImages.digests }}
  {{- $packagesProxy := $context.Values.nodeManager.internal.packagesProxy | default (dict) }}
  {{- $_ := set $tpl_context "packagesProxy" $packagesProxy }}
  {{- if hasKey $context.Values.nodeManager.internal "cloudProvider" }}
    {{- $_ := set $tpl_context "provider" $context.Values.nodeManager.internal.cloudProvider.type }}
  {{- end }}
#!/usr/bin/env bash
set -Eeuo pipefail

BOOTSTRAP_DIR="/var/lib/bashible"
TMPDIR="/opt/deckhouse/tmp"
mkdir -p "${BOOTSTRAP_DIR}" "${TMPDIR}"
export PATH="/opt/deckhouse/bin:/usr/local/bin:$PATH"

  {{- if or (eq $ng.nodeType "CloudEphemeral") (hasKey $ng "staticInstances") }}
exec >"${TMPDIR}/bootstrap.log" 2>&1
  {{- end }}

  {{- $candi := "candi/bashible/lib.sh.tpl" -}}
  {{- $deckhouse := "/deckhouse/candi/bashible/lib.sh.tpl" -}}
  {{- $lib := $context.Files.Get $deckhouse | default ($context.Files.Get $candi) -}}
  {{- $ctx := $tpl_context -}}
  {{- tpl (printf `
  %s
  {{ template "get-phase2" $ }}
  ` $lib) $ctx }}


#prepare_base_d8_binaries
  {{- if $fetch_base_pkgs := $context.Files.Get "candi/bashible/bootstrap/01-bootstrap-prerequisites.sh.tpl" }}
    {{- tpl ( $fetch_base_pkgs ) $tpl_context | nindent 0 }}
  {{- end }}

  {{- if or (eq $ng.nodeType "CloudEphemeral") (hasKey $ng "staticInstances") }}
/opt/deckhouse/bin/tail-log ${TMPDIR}/bootstrap.log &
bootstrap_job_log_pid=$!
  {{- end }}

#run phase2
get_phase2 | bash


if [ -n "${bootstrap_job_log_pid}" ]; then
  kill -9 "${bootstrap_job_log_pid}"
fi

{{- end }}
