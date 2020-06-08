{{- if hasKey . "nodeIP" }}
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-controller-manager
  namespace: kube-system
spec:
  containers:
  - name: kube-controller-manager
    livenessProbe:
      httpGet:
        host: {{ .nodeIP | quote }}
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-scheduler
  namespace: kube-system
spec:
  containers:
  - name: kube-scheduler
    livenessProbe:
      httpGet:
        host: {{ .nodeIP | quote }}
{{- end }}
