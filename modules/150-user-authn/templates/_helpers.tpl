{{- /* We do not need to follow global logic of naming tls secrets if publish API mode is not global */ -}}
{{- define "publish_api_certificate_name" }}
  {{- if eq .Values.userAuthn.publishAPI.https.mode "Global" }}
{{- include "helm_lib_module_https_secret_name" (list . "kubernetes-tls") }}
  {{- else }}
{{- printf "kubernetes-tls-selfsigned" }}
  {{- end }}
{{- end }}


{{- define "publish_api_deploy_certificate" }}
  {{- if .Values.userAuthn.publishAPI.enabled }}
    {{- if eq .Values.userAuthn.publishAPI.https.mode "Global" -}}
      {{- if eq (include "helm_lib_module_https_mode" .) "CertManager" }}
      "not empty string"
      {{- end }}
    {{- else }}
      "not empty string"
    {{- end }}
  {{- end }}
{{- end }}

{{ define "getApiVersion" }}
  {{- if .Capabilities.APIVersions.Has "admissionregistration.k8s.io/v1/ValidatingAdmissionPolicy" }}
apiVersion: admissionregistration.k8s.io/v1
  {{- else if .Capabilities.APIVersions.Has "admissionregistration.k8s.io/v1beta1/ValidatingAdmissionPolicy" }}
  apiVersion: admissionregistration.k8s.io/v1beta1
  {{- else }}
apiVersion: admissionregistration.k8s.io/v1alpha1
  {{- end }}
{{- end }}

