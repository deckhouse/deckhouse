{{- /* We do not need to follow global logic of naming tls secrets if publish API mode is not global */ -}}
{{- define "publish_api_certificate_name" }}
  {{- if eq .Values.controlPlaneManager.apiserver.publishAPI.ingress.https.mode "Global" }}
{{- include "helm_lib_module_https_secret_name" (list . "kubernetes-tls") }}
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

{{- /*
  Global mode needs its own cert-manager Certificate (and therefore its own secret name) only
  when cert-manager actually issues it: the Gateway API path is validated through a separate
  ClusterIssuer with its own ACME HTTP01 solver (see
  helm_lib_module_https_cert_manager_cluster_issuer_name_for_gateway_api), so ingress's
  certificate can't simply be reused there. In CustomCertificate mode there is no issuance or
  per-mechanism validation at all — it's the same static certificate data either way — so the
  Gateway path reuses the exact secret ingress already has (publish_api_certificate_name),
  instead of keeping a second, redundant copy of it.
*/ -}}
{{- define "publish_api_http_route_certificate_name" }}
  {{- if and (eq .Values.controlPlaneManager.apiserver.publishAPI.ingress.https.mode "Global") (eq (include "helm_lib_module_https_mode" .) "CertManager") }}
{{- include "helm_lib_module_https_secret_name" (list . "kubernetes-httproute-tls") }}
  {{- else }}
{{- include "publish_api_certificate_name" . }}
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