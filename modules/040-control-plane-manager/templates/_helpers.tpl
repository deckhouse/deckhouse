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
