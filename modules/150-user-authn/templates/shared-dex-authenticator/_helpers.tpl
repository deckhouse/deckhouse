{{- /* The shared dex-authenticator: two Deployments of one image in the namespace of the module, each serving the
       DexAuthenticator objects of many namespaces as the clients of a table (patch 018 of the image). The system
       instance is for the objects of the d8-* namespaces, the other one for all the rest, so that the sign-in to the
       platform does not share a process with the applications of users. */ -}}

{{- /* The names of the instances. Usage: range (include "shared_dex_authenticator_instances" . | fromYamlArray) */ -}}
{{- define "shared_dex_authenticator_instances" -}}
- dex-authenticator-system
- dex-authenticator
{{- end -}}

{{- /* "true" while the instances are rendered. Until DexAuthenticator objects are switched to them, nothing in a
       cluster sets the value: only the tests of the templates do. */ -}}
{{- define "shared_dex_authenticator_enabled" -}}
  {{- if dig "sharedDexAuthenticator" "enabled" false .Values.userAuthn.internal -}}
true
  {{- end -}}
{{- end -}}

{{- /* The labels of the pods of an instance and of what selects them. Usage: include "..." $instance */ -}}
{{- define "shared_dex_authenticator_pod_labels" -}}
app: shared-dex-authenticator
instance: {{ . }}
{{- end -}}

{{- define "shared_dex_authenticator_resources" }}
cpu: 100m
memory: 128Mi
{{- end }}

{{- define "shared_dex_authenticator_redis_resources" }}
cpu: 20m
memory: 64Mi
{{- end }}
