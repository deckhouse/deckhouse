{{- /* Usage: {{ include "helm_lib_flow_schema_manifest" (list . "cluster-low") }} */ -}}
{{- define "helm_lib_flow_schema_manifest" }}
  {{- $context := index . 0 }}
  {{- $priorityLevelConfiguration := index . 1 }}
  {{- $kubernetesSemVer := semver $context.Values.global.discovery.kubernetesVersion }}
---
  {{- include "helm_lib_flow_schema_apiversion" (list $context) }}
kind: FlowSchema
metadata:
  name: {{ $context.Chart.Name }}-flowschema
  {{- include "helm_lib_module_labels" (list $context) | nindent 2 }}
spec:
  distinguisherMethod:
    type: ByUser
  matchingPrecedence: 1000
  priorityLevelConfiguration:
    name: {{ $priorityLevelConfiguration }}
  rules:
    - resourceRules:
        - apiGroups:
            - '*'
          clusterScope: true
          namespaces:
            - '*'
          resources:
            - '*'
          verbs:
            - 'list'
      subjects:
        - group:
            name: system:serviceaccounts:d8-{{ $context.Chart.Name }}
          kind: Group
{{- end }}

{{- /* Usage: {{ include "helm_lib_flow_schema_apiversion" (list .) }} */ -}}
{{- define "helm_lib_flow_schema_apiversion" }}
  {{- $context := index . 0 }}
  {{- $kubernetesSemVer := semver $context.Values.global.discovery.kubernetesVersion }}
  {{- if ge $kubernetesSemVer.Minor "26" }}
apiVersion: flowcontrol.apiserver.k8s.io/v1beta3
  {{- else }}
apiVersion: flowcontrol.apiserver.k8s.io/v1beta1
  {{- end }}
{{- end }}
