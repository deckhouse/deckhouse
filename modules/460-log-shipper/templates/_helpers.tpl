{{- define "vectorEnv" }}
- name: VECTOR_SELF_NODE_NAME
  valueFrom:
    fieldRef:
      fieldPath: spec.nodeName
- name: VECTOR_SELF_POD_NAME
  valueFrom:
    fieldRef:
      fieldPath: metadata.name
- name: VECTOR_SELF_POD_NAMESPACE
  valueFrom:
    fieldRef:
      fieldPath: metadata.namespace
{{- end }}
