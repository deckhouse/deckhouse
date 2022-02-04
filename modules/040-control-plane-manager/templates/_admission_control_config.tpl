{{- define "admissionControlConfig" }}
apiVersion: apiserver.config.k8s.io/v1
kind: AdmissionConfiguration
plugins:
- name: EventRateLimit
  path: /etc/kubernetes/deckhouse/extra-files/event-rate-limit-config.yaml
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
