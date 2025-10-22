{{- define "resources" }}
resources:
  requests:
    memory: {{ pluck .Values.web.env .Values.resources.requests.memory | first | default .Values.resources.requests.memory._default }}
  limits:
    memory: {{ pluck .Values.web.env .Values.resources.limits.memory | first | default .Values.resources.limits.memory._default }}
{{- end }}
