{{- /* The S3 storage in use: System needs the system-s3 module, without it
  the module keeps versions in the pod only and raises an alert (see
  monitoring/prometheus-rules/system-s3.tpl). A render error would not reach
  the user and could hold up the Deckhouse queue. Without the documentation
  pod there is nothing to keep, so no bucket either. The helpers live here,
  so the alert template and the generator of the alert documentation see
  them too. */}}
{{- define "documentation_s3_requested" }}{{ dig "moduleStorage" "s3" "type" "Disabled" .Values.documentation }}{{ end }}
{{- define "documentation_s3" }}
  {{- $requested := include "documentation_s3_requested" . }}
  {{- if not (and .Values.global.modules.publicDomainTemplate .Values.global.clusterIsBootstrapped) }}Disabled
  {{- else if and (eq $requested "System") (has "system-s3" .Values.global.enabledModules) }}System
  {{- else if eq $requested "External" }}External
  {{- else }}Disabled
  {{- end }}
{{- end }}
{{- /* The size of the bucket, given to docs-builder for the eviction: of the
  SystemBucket, or of the part of the user's bucket under the prefix. */}}
{{- define "documentation_s3_size" }}{{ if eq (include "documentation_s3" .) "External" }}{{ .Values.documentation.moduleStorage.s3.external.size }}{{ else }}5Gi{{ end }}{{ end }}
