{{- define "resources" }}
resources:
  requests:
    memory: {{ pluck .Values.web.env .Values.resources.requests.memory | first | default .Values.resources.requests.memory._default }}
{{- end }}

{{- define "vrouter_envs" }}
- name: VROUTER_DEFAULT_GROUP
  value: {{ .Values.vrouter.defaultGroup | quote }}
- name: VROUTER_DEFAULT_CHANNEL
  value: {{ pluck .Values.web.env .Values.vrouter.defaultChannel | first | default .Values.vrouter.defaultChannel._default | quote }}
- name: VROUTER_SHOW_LATEST_CHANNEL
  value: {{ .Values.vrouter.showLatestChannel | quote }}
- name: VROUTER_LISTEN_PORT
  value: "8082"
- name: VROUTER_LOG_LEVEL
  value: {{ pluck .Values.web.env .Values.vrouter.logLevel | first | default .Values.vrouter.logLevel._default | quote }}
- name: VROUTER_PATH_STATIC
  value: {{ pluck .Values.web.env .Values.vrouter.pathStatic | first | default .Values.vrouter.pathStatic._default | quote }}
- name: VROUTER_LOCATION_VERSIONS
  value: {{ .Values.vrouter.locationVersions | quote }}
- name: VROUTER_PATH_CHANNELS_FILE
  value: {{ pluck .Values.web.env .Values.vrouter.pathChannelsFile | first | default .Values.vrouter.pathChannelsFile._default | quote }}
- name: VROUTER_PATH_TPLS
  value: {{ pluck .Values.web.env .Values.vrouter.pathTpls | first | default .Values.vrouter.pathTpls._default | quote }}
- name: VROUTER_I18N_TYPE
  value: {{ .Values.vrouter.i18nType | quote }}
- name: VROUTER_URL_VALIDATION
  value: {{ pluck .Values.web.env .Values.vrouter.urlValidation | first | default .Values.vrouter.urlValidation._default | quote }}
{{- end }}

{{- define "readiness_probe" }}
failureThreshold: 5
periodSeconds: 10
timeoutSeconds: 5
{{- end }}
{{- define "liveness_probe" }}
failureThreshold: 10
periodSeconds: 10
timeoutSeconds: 5
{{- end }}
{{- define "startup_probe" }}
failureThreshold: 10
periodSeconds: 10
timeoutSeconds: 5
{{- end }}
