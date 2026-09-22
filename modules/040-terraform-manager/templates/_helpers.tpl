{{- define "terraform_manager_image" }}
{{- $cloudProvider := (.Values.global.clusterConfiguration.cloud.provider | lower ) -}}
{{- /* In-tree providers bake candi+plugin into their own terraformManager<Provider> image */ -}}
{{- $image := include "helm_lib_module_image_no_fail" (list . (printf "terraformManager%s" ($cloudProvider | title))) -}}
{{- if not $image -}}
{{- /* External providers have no per-provider image; the bundle with candi+plugin is downloaded by dhctl at runtime */ -}}
{{- $image = include "helm_lib_module_image" (list . "terraformManager") -}}
{{- end -}}
{{ $image }}
{{- end }}
