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
        scheme: HTTPS
        port: 10257
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
        scheme: HTTPS
        port: 10259
{{- end }}
