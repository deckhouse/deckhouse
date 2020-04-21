{{- if hasKey . "nodeIP" }}
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-controller-manager
  namespace: kube-system
  containers:
  - name: kube-controller-manager
    readinessProbe:
      host: {{ .nodeIP | quote }}
    livenessProbe:
      host: {{ .nodeIP | quote }}
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-scheduler
  namespace: kube-system
  containers:
  - name: kube-scheduler
    readinessProbe:
      host: {{ .nodeIP | quote }}
    livenessProbe:
      host: {{ .nodeIP | quote }}
{{- end }}
