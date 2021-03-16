{{- range $key, $value := index .k8s .kubernetesVersion "bashible" "ubuntu" }}
  {{- $ubuntuVersion := toString $key }}
  {{- if or $value.kernel.aws.desiredVersion $value.kernel.aws.allowedPattern }}
if bb-is-ubuntu-version? {{ $ubuntuVersion }} ; then
  cat <<EOF > /var/lib/bashible/kernel_version_config_by_cloud_provider
desired_version={{ $value.kernel.aws.desiredVersion | quote }}
allowed_versions_pattern={{ $value.kernel.aws.allowedPattern | quote }}
EOF
fi
  {{- end }}
{{- end }}
