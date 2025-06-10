{{- define "admissionControlConfig" }}
apiVersion: apiserver.config.k8s.io/v1
kind: AdmissionConfiguration
plugins:
- name: EventRateLimit
  path: /etc/kubernetes/deckhouse/extra-files/event-rate-limit-config.yaml
- name: ValidatingAdmissionWebhook
  configuration:
    apiVersion: apiserver.config.k8s.io/v1
    kind: WebhookAdmissionConfiguration
    kubeConfigFile: /etc/kubernetes/deckhouse/extra-files/webhook-admission-config.yaml
{{- end }}

{{- define "eventRateLimitAdmissionConfig" }}
apiVersion: eventratelimit.admission.k8s.io/v1alpha1
kind: Configuration
limits:
- type: Namespace
  qps: 50
  burst: 100
  cacheSize: 2000
{{- end }}

{{- define "webhookAdmissionConfig" }}
apiVersion: v1
kind: Config
users:
- name: "*"
  user:
    client-certificate-data: {{ .Values.controlPlaneManager.internal.admissionWebhookClient.cert | b64enc }}
    client-key-data: {{ .Values.controlPlaneManager.internal.admissionWebhookClient.key | b64enc }}
{{- end }}
