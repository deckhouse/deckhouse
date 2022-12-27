{{- /* Usage: {{ include "helm_lib_flow_schema_manifest" (list . "cluster-low") }} */ -}}
{{- define "helm_lib_flow_schema_manifest" }}
{{- $context := index . 0 }}
{{- $priorityLevelConfiguration := index . 1 }}

apiVersion: flowcontrol.apiserver.k8s.io/v1beta1
kind: FlowSchema
metadata:
  name: {{ $context.Chart.Name }}-flowschema
  namespace: d8-{{ $context.Chart.Name }}
  {{- include "helm_lib_module_labels" (list $context) | nindent 2 }}
spec:
  distinguisherMethod:
    type: ByUser
  matchingPrecedence: 1000
  priorityLevelConfiguration:
    name: {{ $priorityLevelConfiguration }}/
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
