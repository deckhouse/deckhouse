{{- define "template.service" }}
  {{- $annotations := (.annotations | default (dict)) }}
  {{- $type := (.type | default "ClusterIP") }}
  {{- with .context }}
---
apiVersion: v1
kind: Service
metadata:
  name: nginx
  namespace: {{ include "helper.namespace" . }}
  labels:
    heritage: antiopa
    module: {{ .Chart.Name }}
    app: nginx
{{- if gt (len $annotations) 0 }}
  annotations:
{{ $annotations | toYaml | indent 4 }}
{{- end }}
spec:
  type: {{ $type }}
  externalTrafficPolicy: Local
  ports:
  - name: http
    port: 80
    targetPort: 80
    protocol: TCP
{{- if and ( eq $type "NodePort") (.nodePortHTTP) }}
    nodePort: {{ .nodePortHTTP }}
{{- end }}
  - name: https
    port: 443
    targetPort: 443
    protocol: TCP
{{- if and ( eq $type "NodePort") (.nodePortHTTPS) }}
    nodePort: {{ .nodePortHTTPS }}
{{- end }}
  selector:
    app: nginx
  {{- end }}
{{- end }}
