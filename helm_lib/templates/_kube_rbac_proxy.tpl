{{- /* Usage: {{ include "helm_lib_kube_rbac_proxy_ca_certificate" (list . "namespace") }} */ -}}
{{- /* Renders configmap with kube-rbac-proxy CA certificate which uses to verify the kube-rbac-proxy clients. */ -}}
{{- define "helm_lib_kube_rbac_proxy_ca_certificate" -}}
  {{- $context := index . 0 }} {{- /* Dot object (.) with .Values, .Chart, etc */ -}}
  {{- $namespace := index . 1 }} {{- /* Namespace where will be created configmap  */ -}}
---
apiVersion: v1
data:
  ca.crt: |
    {{ $context.Values.global.internal.modules.kubeRBACProxyCA.cert | nindent 4 }}
kind: ConfigMap
metadata:
  annotations:
    kubernetes.io/description: |
      Contains a CA bundle that can be used to verify the kube-rbac-proxy clients.
  {{- include "helm_lib_module_labels" (list $context) | nindent 2 }}
  name: kube-rbac-proxy-ca.crt
  namespace: {{ $namespace }}
{{- end }}
