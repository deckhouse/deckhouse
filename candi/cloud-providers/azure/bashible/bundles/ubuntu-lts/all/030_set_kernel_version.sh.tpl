{{- range $key, $value := index .k8s .kubernetesVersion "bashible" "ubuntu" }}
  {{- $ubuntuVersion := toString $key }}
  {{- if or $value.kernel.azure.desiredVersion $value.kernel.azure.allowedPattern }}
if bb-is-ubuntu-version? {{ $ubuntuVersion }} ; then
  cat <<EOF > /var/lib/bashible/kernel_version_config_by_cloud_provider
desired_version={{ $value.kernel.azure.desiredVersion | quote }}
allowed_versions_pattern={{ $value.kernel.azure.allowedPattern | quote }}
EOF
fi
  {{- end }}
{{- end }}
