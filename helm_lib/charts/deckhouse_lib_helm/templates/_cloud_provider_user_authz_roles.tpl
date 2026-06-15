{{- /* Usage: {{- include "helm_lib_cloud_provider_user_authz_cluster_roles" (list . $config) }} */ -}}
{{- /* Renders user-authz ClusterRoles for provider-specific cloud resources. */ -}}
{{- /* Includes User and ClusterAdmin ClusterRoles. */ -}}
{{- /* Supported configuration parameters: */ -}}
{{- /* + providerName (required) — provider name segment used in ClusterRole names. */ -}}
{{- /* + instanceClassResource (required) — Deckhouse instance class resource name granted by the rules. */ -}}
{{- /* + capiResources (optional, default: `[]`) — CAPI infrastructure resource names granted by the rules. */ -}}
{{- /* + additionalUserRules (optional, default: `[]`) — extra rules appended to the User ClusterRole. */ -}}
{{- /* + additionalClusterAdminRules (optional, default: `[]`) — extra rules appended to the ClusterAdmin ClusterRole. */ -}}
{{- define "helm_lib_cloud_provider_user_authz_cluster_roles" -}}
  {{- $context := index . 0 -}}
  {{- $config := index . 1 -}}
  {{- $providerName := required "helm_lib_cloud_provider_user_authz_cluster_roles: providerName is required" (get $config "providerName") -}}
  {{- $instanceClassResource := required "helm_lib_cloud_provider_user_authz_cluster_roles: instanceClassResource is required" (get $config "instanceClassResource") -}}
  {{- $capiResources := dig "capiResources" (list) $config -}}
  {{- $additionalUserRules := dig "additionalUserRules" (list) $config -}}
  {{- $additionalClusterAdminRules := dig "additionalClusterAdminRules" (list) $config -}}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  annotations:
    user-authz.deckhouse.io/access-level: User
  name: d8:user-authz:{{ $providerName }}:user
  {{- include "helm_lib_module_labels" (list $context) | nindent 2 }}
rules:
- apiGroups:
  - deckhouse.io
  resources:
  - {{ $instanceClassResource }}
  verbs:
  - get
  - list
  - watch
{{- if $capiResources }}
- apiGroups:
  - infrastructure.cluster.x-k8s.io
  resources:
  {{- range $capiResources }}
  - {{ . }}
  {{- end }}
  verbs:
  - get
  - list
  - watch
{{- end }}
{{- with $additionalUserRules }}
{{ toYaml . }}
{{- end }}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  annotations:
    user-authz.deckhouse.io/access-level: ClusterAdmin
  name: d8:user-authz:{{ $providerName }}:cluster-admin
  {{- include "helm_lib_module_labels" (list $context) | nindent 2 }}
rules:
- apiGroups:
  - deckhouse.io
  resources:
  - {{ $instanceClassResource }}
  verbs:
  - create
  - delete
  - deletecollection
  - patch
  - update
{{- if $capiResources }}
- apiGroups:
  - infrastructure.cluster.x-k8s.io
  resources:
  {{- range $capiResources }}
  - {{ . }}
  {{- end }}
  verbs:
  - patch
  - update
{{- end }}
{{- with $additionalClusterAdminRules }}
{{ toYaml . }}
{{- end }}
{{- end -}}
