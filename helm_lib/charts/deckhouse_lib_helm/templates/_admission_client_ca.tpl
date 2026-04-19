{{- /* Usage: {{ include "helm_lib_admission_webhook_client_ca_certificate" (list . "namespace") }} */ -}}
{{- /* Renders configmap with admission webhook client CA certificate which uses to verify the AdmissionReview requests. */ -}}
{{- define "helm_lib_admission_webhook_client_ca_certificate" -}}
{{- /* Template context with .Values, .Chart, etc */ -}}
{{- /* Namespace where CA configmap will be created  */ -}}
  {{- $context := index . 0 }}
  {{- $namespace := index . 1 }}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: admission-client-ca.crt
  namespace: {{ $namespace }}
  annotations:
    kubernetes.io/description: |
      Contains a CA bundle that can be used to verify the admission webhook client.
  {{- include "helm_lib_module_labels" (list $context) | nindent 2 }}
data:
  ca.crt: |
    {{ $context.Values.global.internal.modules.admissionWebhookClientCA.cert | nindent 4 }}
{{- end }}
