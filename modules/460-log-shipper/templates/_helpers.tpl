{{- define "vectorEnv" }}
  {{- if .Values.logShipper.debug }}
- name: VECTOR_LOG
  value: debug
- name: RUST_BACKTRACE
  value: full
  {{- end }}
- name: VECTOR_SELF_POD_NAME
  valueFrom:
    fieldRef:
      fieldPath: metadata.name
- name: VECTOR_SELF_POD_NAMESPACE
  valueFrom:
    fieldRef:
      fieldPath: metadata.namespace
- name: VECTOR_HOST_IP
  valueFrom:
    fieldRef:
      fieldPath: status.hostIP
- name: VECTOR_HOSTNAME
  valueFrom:
    fieldRef:
      fieldPath: spec.nodeName
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
- name: localtime
  mountPath: /etc/localtime
  readOnly: true
{{- end }}
