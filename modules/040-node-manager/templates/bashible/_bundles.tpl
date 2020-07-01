{{- /* deepCopy imitation */ -}}
{{- define "bundles_tpl_context_common_yaml" -}}
  {{- $context := . }}

  {{- $normal := dict }}
  {{- $_ := set $normal "bootstrapTokenPath" "/var/lib/bashible/bootstrap-token" }}
  {{- $_ := set $normal "apiserverEndpoints" $context.Values.nodeManager.internal.clusterMasterAddresses }}
  {{- $_ := set $normal "clusterDomain" $context.Values.global.discovery.clusterDomain }}
  {{- $_ := set $normal "clusterDNSAddress" $context.Values.global.discovery.clusterDNSAddress }}
  {{- $_ := set $normal "kubernetesCA" $context.Values.nodeManager.internal.kubernetesCA }}

  {{- $tpl_context_common := dict }}
  {{- $_ := set $tpl_context_common "runType" "Normal" }}
  {{- $_ := set $tpl_context_common "Template" $context.Template }}
  {{- $_ := set $tpl_context_common "normal" $normal }}

  {{- $tpl_context_common | toYaml }}
{{- end -}}

{{- define "bundles_common_steps_pattern" -}}
  {{- printf "candi/bashible/common-steps/%s/*.sh.tpl" (first .) }}
{{- end -}}

{{- define "bundles_bundle_steps_pattern" -}}
  {{- printf "candi/bashible/bundles/%s/%s/*.sh.tpl" (first .) (last .) }}
{{- end -}}

{{- define "bundles_cloud_provider_common_steps_pattern" -}}
  {{- printf (list "candi/cloud-providers/" (index . 0) "/bashible/common-steps/%s/*.sh.tpl" | join "") (index . 1) }}
{{- end -}}

{{- define "bundles_cloud_provider_bundle_steps_pattern" -}}
  {{- printf (list "candi/cloud-providers/" (index . 0) "/bashible/bundles/%s/%s/*.sh.tpl" | join "") (index . 1) (index . 2) }}
{{- end -}}

{{- define "bundles_validate_step_file" -}}
  {{- $step_file := . -}}
  {{- $step_file_name := base $step_file -}}

  {{- if not (regexMatch "^[0-9]+_" $step_file_name) -}}
    {{- fail (printf "ERROR: Can't handle bashible step template %s. File name must match the pattern: ^[0-9]+_" $step_file) -}}
  {{- end -}}
{{- end -}}

{{- define "bundles_rendered_steps_node_group" -}}
  {{- $context := index . 0 -}}
  {{- $bundle  := index . 1 -}}
  {{- $ng      := index . 2 -}}

  {{- $tpl_context := (include "bundles_tpl_context_common_yaml" $context | fromYaml) }}
  {{- $_ := set $tpl_context "bundle" $bundle }}
  {{- $_ := set $tpl_context "kubernetesVersion" $ng.kubernetesVersion }}
  {{- $_ := set $tpl_context "nodeGroup" $ng }}

  {{- range $step_file, $_ := $context.Files.Glob (include "bundles_common_steps_pattern" (list "node-group")) }}
    {{- include "bundles_validate_step_file" $step_file }}
{{ trimSuffix ".tpl" (base $step_file) }}: {{ tpl ($context.Files.Get $step_file) $tpl_context | b64enc }}
  {{- end }}

  {{- range $step_file, $_ := $context.Files.Glob (include "bundles_bundle_steps_pattern"  (list $bundle "node-group")) }}
    {{- include "bundles_validate_step_file" $step_file }}
{{ trimSuffix ".tpl" (base $step_file) }}: {{ tpl ($context.Files.Get $step_file) $tpl_context | b64enc }}
  {{- end }}

  {{- if hasKey $context.Values.nodeManager.internal "cloudProvider" }}
    {{- $cloud_provider := $context.Values.nodeManager.internal.cloudProvider }}
    {{- $_ := set $tpl_context "cloudProvider" $cloud_provider }}

    {{- range $step_file, $_ := $context.Files.Glob (include "bundles_cloud_provider_common_steps_pattern" (list $cloud_provider.type "node-group")) }}
      {{- include "bundles_validate_step_file" $step_file }}
{{ trimSuffix ".tpl" (base $step_file) }}: {{ tpl ($context.Files.Get $step_file) $tpl_context | b64enc }}
    {{- end }}

    {{- range $step_file, $_ := $context.Files.Glob (include "bundles_cloud_provider_bundle_steps_pattern" (list $cloud_provider.type $bundle "node-group")) }}
      {{- include "bundles_validate_step_file" $step_file }}
{{ trimSuffix ".tpl" (base $step_file) }}: {{ tpl ($context.Files.Get $step_file) $tpl_context | b64enc }}
    {{- end }}

  {{- end }}
{{- end -}}

{{- define "bundles_rendered_steps_all" -}}
  {{- $context := index . 0 -}}
  {{- $bundle  := index . 1 -}}
  {{- $kubernetes_version := index . 2 -}}

  {{- $tpl_context := (include "bundles_tpl_context_common_yaml" $context | fromYaml) }}
  {{- $_ := set $tpl_context "bundle" $bundle }}
  {{- $_ := set $tpl_context "kubernetesVersion" $kubernetes_version }}


  {{- range $step_file, $_ := $context.Files.Glob (include "bundles_common_steps_pattern" (list "all")) }}
    {{- include "bundles_validate_step_file" $step_file }}
{{ trimSuffix ".tpl" (base $step_file) }}: {{ tpl ($context.Files.Get $step_file) $tpl_context | b64enc }}
  {{- end }}

  {{- range $step_file, $_ := $context.Files.Glob (include "bundles_bundle_steps_pattern"  (list $bundle "all")) }}
    {{- include "bundles_validate_step_file" $step_file }}
{{ trimSuffix ".tpl" (base $step_file) }}: {{ tpl ($context.Files.Get $step_file) $tpl_context | b64enc }}
  {{- end }}

  {{- if hasKey $context.Values.nodeManager.internal "cloudProvider" }}
    {{- $cloud_provider := $context.Values.nodeManager.internal.cloudProvider }}
    {{- $_ := set $tpl_context "cloudProvider" $cloud_provider }}

    {{- range $step_file, $_ := $context.Files.Glob (include "bundles_cloud_provider_common_steps_pattern" (list $cloud_provider.type "all")) }}
      {{- include "bundles_validate_step_file" $step_file }}
{{ trimSuffix ".tpl" (base $step_file) }}: {{ tpl ($context.Files.Get $step_file) $tpl_context | b64enc }}
    {{- end }}

    {{- range $step_file, $_ := $context.Files.Glob (include "bundles_cloud_provider_bundle_steps_pattern" (list $cloud_provider.type $bundle "all")) }}
      {{- include "bundles_validate_step_file" $step_file }}
{{ trimSuffix ".tpl" (base $step_file) }}: {{ tpl ($context.Files.Get $step_file) $tpl_context | b64enc }}
    {{- end }}

  {{- end }}
{{- end -}}
