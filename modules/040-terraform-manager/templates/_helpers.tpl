{{- define "terraform_manager_image" }}
{{- $cloudProvider := (.Values.global.clusterConfiguration.cloud.provider | lower ) -}}
{{- $imageTag := (pluck (printf "terraformManager%s" ($cloudProvider | title)) .Values.global.modulesImages.tags.terraformManager | first ) -}}
terraform-manager-{{ $cloudProvider }}:{{ $imageTag }}
{{- end }}
