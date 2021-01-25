{{- define "is_istio_in_use" -}}
  {{- $context := . -}}

  {{- if ($context.Values.global.enabledModules | has "istio") -}}
    {{- range $crd := $context.Values.ingressNginx.internal.ingressControllers -}}
      {{- if $crd.spec.enableIstioSidecar -}}
        not empty string
      {{- end -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
