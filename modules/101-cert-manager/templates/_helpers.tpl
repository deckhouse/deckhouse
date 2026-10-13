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

{{- /*
  cert_manager.webhook_replicas returns the replica count of the webhook: one per master in HA mode,
  as helm_lib_deployment_on_master_strategy_and_replicas_for_ha does, and one otherwise. The strategy
  is set by the Deployment itself, because the webhook surges instead of stopping replicas first.
*/ -}}
{{- define "cert_manager.webhook_replicas" -}}
  {{- if (include "helm_lib_ha_enabled" .) -}}
    {{- if gt (index .Values.global.discovery "clusterMasterCount" | int) 0 -}}
      {{- index .Values.global.discovery "clusterMasterCount" -}}
    {{- else if gt (index .Values.global.discovery.d8SpecificNodeCountByRole "master" | int) 0 -}}
      {{- index .Values.global.discovery.d8SpecificNodeCountByRole "master" -}}
    {{- else -}}
      2
    {{- end -}}
  {{- else -}}
    1
  {{- end -}}
{{- end -}}
