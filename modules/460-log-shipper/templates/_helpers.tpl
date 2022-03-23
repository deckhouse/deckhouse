{{- define "vectorEnv" }}
  {{- if .Values.logShipper.debug }}
# Migrate to VECTOR_LOG after upgrade to 0.18.
- name: LOG
  value: debug
- name: VECTOR_LOG
  value: debug
- name: RUST_BACKTRACE
  value: full
  {{- end }}
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

{{- define "vectorMounts" }}
- name: vector-data-dir
  mountPath: "/vector-data"
- name: vector-config-dir
  mountPath: /etc/vector/dynamic
- name: vector-sample-config-dir
  mountPath: /etc/vector/default/defaults.json
  readOnly: true
  subPath: defaults.json
{{- end }}
