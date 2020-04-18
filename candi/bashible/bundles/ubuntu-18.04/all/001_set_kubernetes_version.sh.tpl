{{ if eq .kubernetesVersion "1.14" }}
  echo 1.14.10-00 > /var/lib/bashible/kubernetes-version
{{ else if eq .kubernetesVersion "1.15" }}
  echo 1.15.11-00 > /var/lib/bashible/kubernetes-version
{{ else if eq .kubernetesVersion "1.16" }}
  echo 1.16.8-00 > /var/lib/bashible/kubernetes-version
{{ else }}
  {{ fail (printf "Unsupported kubernetes version: %s" .kubernetesVersion) }}
{{ end }}
