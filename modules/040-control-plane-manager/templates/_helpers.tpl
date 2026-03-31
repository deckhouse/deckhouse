<<<<<<< HEAD
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
=======
{{- /* We do not need to follow global logic of naming tls secrets if publish API mode is not global */ -}}
{{- define "publish_api_certificate_name" }}
  {{- if eq .Values.controlPlaneManager.publishAPI.ingress.https.mode "Global" }}
{{- include "helm_lib_module_https_secret_name" (list . "kubernetes-tls") }}
  {{- else }}
{{- printf "kubernetes-tls-selfsigned" }}
  {{- end }}
{{- end }}


{{- define "publish_api_deploy_certificate" }}
  {{- if .Values.controlPlaneManager.publishAPI.ingress.enabled }}
    {{- if eq .Values.controlPlaneManager.publishAPI.ingress.https.mode "Global" -}}
      {{- if eq (include "helm_lib_module_https_mode" .) "CertManager" }}
      "not empty string"
      {{- end }}
    {{- else }}
      "not empty string"
    {{- end }}
  {{- end }}
{{- end }}
>>>>>>> 3aeb8370db (test get values of another module)
