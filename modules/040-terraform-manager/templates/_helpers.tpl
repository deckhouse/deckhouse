{{- define "terraform_manager_image" }}
{{- $cloudProvider := (.Values.global.clusterConfiguration.cloud.provider | lower ) -}}
{{- $image := include "helm_lib_module_image" (list . (printf "terraformManager%s" ($cloudProvider | title))) -}}
{{ $image }}
{{- end }}
