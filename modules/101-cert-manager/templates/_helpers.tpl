{{- /*
  cert_manager.gateway_api_crd_serves returns a non-empty string when the named Gateway API CRD
  declares the given version as served, and an empty one otherwise.

  The version is a parameter rather than part of the rule: which one cert-manager needs is the
  caller's business, and every caller today needs the stable v1. Deprecated versions never reach
  this list -- discovery drops them -- and which version is the storage one is the API server's
  business, so neither is consulted here.

  Usage: include "cert_manager.gateway_api_crd_serves" (list . "gateways.gateway.networking.k8s.io" "v1")
*/ -}}
{{- define "cert_manager.gateway_api_crd_serves" -}}
  {{- $context := index . 0 -}}
  {{- $crdName := index . 1 -}}
  {{- $version := index . 2 -}}
  {{- range dig "gatewayAPICRDs" (list) $context.Values.global.discovery -}}
    {{- if eq .name $crdName -}}
      {{- range .versions -}}
        {{- if and (eq .name $version) .served -}}served{{- end -}}
      {{- end -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
