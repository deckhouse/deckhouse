{{- /* Usage: {{ include "publish_api_certificate_name" . }} */ -}}
{{- /* or:    {{ include "publish_api_certificate_name" (list . "own_secret_name_prefix_for_gateway_api") }} */ -}}
{{- /* We do not need to follow global logic of naming tls secrets if publish API mode is not global */ -}}
{{- define "publish_api_certificate_name" }}
  {{- $context := . }}
  {{- $own_secret_name_prefix_for_gateway_api := "" }}
  {{- if kindIs "slice" . }}
    {{- $context = index . 0 }}
    {{- $own_secret_name_prefix_for_gateway_api = index . 1 }}
  {{- end }}
  {{- if eq $context.Values.controlPlaneManager.apiserver.publishAPI.ingress.https.mode "Global" }}
    {{- if $own_secret_name_prefix_for_gateway_api }}
{{- include "helm_lib_module_https_secret_name" (list $context "kubernetes-tls" $own_secret_name_prefix_for_gateway_api) }}
    {{- else }}
{{- include "helm_lib_module_https_secret_name" (list $context "kubernetes-tls") }}
    {{- end }}
  {{- else }}
{{- printf "kubernetes-tls-selfsigned" }}
  {{- end }}
{{- end }}


{{- define "publish_api_deploy_certificate" }}
  {{- if .Values.controlPlaneManager.apiserver.publishAPI.ingress.enabled }}
    {{- if eq .Values.controlPlaneManager.apiserver.publishAPI.ingress.https.mode "Global" -}}
      {{- if eq (include "helm_lib_module_https_mode" .) "CertManager" }}
      "not empty string"
      {{- end }}
    {{- else if eq .Values.controlPlaneManager.apiserver.publishAPI.ingress.https.mode "SelfSigned" }}
      {{- if .Values.global.enabledModules | has "cert-manager" }}
      "not empty string"
      {{- end }}
    {{- end }}
  {{- end }}
{{- end }}

{{/*
  Returns "true" when the control-plane-manager DaemonSet must set NODE_ADMIN_KUBECONFIG=false so the
  controller removes the /root/.kube/config -> admin.conf symlink.

  Only applies if user-authz is enabled and controlPlaneManager.rootKubeconfigSymlink is false (default is true).
  If user-authz is disabled, the symlink is not driven by this setting (env is not set to false).
*/}}
{{- define "cpm.disableRootKubeconfigSymlink" -}}
{{- $mods := $.Values.global.enabledModules | default list -}}
{{- $wantSymlink := dig "controlPlaneManager" "rootKubeconfigSymlink" true ($.Values | merge (dict)) -}}
{{- if and (has "user-authz" $mods) (eq $wantSymlink false) -}}
{{- print "true" -}}
{{- end -}}
{{- end -}}