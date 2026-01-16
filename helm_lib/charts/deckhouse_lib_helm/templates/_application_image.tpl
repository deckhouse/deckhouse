{{- /* Usage: {{ include "helm_lib_application_image" (list . "<image-name>") }} */ -}}
{{- /* returns image name in format "registry/package@digest" */ -}}
{{- define "helm_lib_application_image" }}
  {{- $context := index . 0 }}

  {{- $image := index . 1 | trimAll "\"" }}
  {{- $imageDigest := index $context.Runtime.Package.Digests $image }}
  {{- if not $imageDigest }}
  {{- fail (printf "Image %s has no digest" $image) }}
  {{- end }}

  {{- $registryBase := $context.Runtime.Package.Registry.repository }}
  {{- if not $registryBase }}
  {{- fail "Registry base is not set" }}
  {{- end }}

  {{- $packageName := $context.Runtime.Package.Name }}
  {{- if not $packageName }}
  {{- fail "Package name is not set" }}
  {{- end }}

  {{- printf "%s/%s@%s" $registryBase $packageName $imageDigest }}
{{- end }}
