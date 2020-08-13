---
apiVersion: v1
kind: Pod
metadata:
  name: kube-controller-manager
  namespace: kube-system
spec:
  containers:
  - name: kube-controller-manager
    readinessProbe:
      httpGet:
{{- if hasKey . "nodeIP" }}
        host: {{ .nodeIP | quote }}
{{- end }}
        path: /healthz
        port: 10257
        scheme: HTTPS
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-scheduler
  namespace: kube-system
spec:
  containers:
  - name: kube-scheduler
    readinessProbe:
      httpGet:
{{- if hasKey . "nodeIP" }}
        host: {{ .nodeIP | quote }}
{{- end }}
        path: /healthz
        port: 10259
        scheme: HTTPS
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-apiserver
  namespace: kube-system
spec:
  containers:
  - name: kube-apiserver
    readinessProbe:
      httpGet:
{{- if hasKey . "nodeIP" }}
        host: {{ .nodeIP | quote }}
{{- end }}
        path: /healthz
        port: 6443
        scheme: HTTPS
---
apiVersion: v1
kind: Pod
metadata:
  name: etcd
  namespace: kube-system
spec:
  containers:
  - name: etcd
    readinessProbe:
      httpGet:
        host: 127.0.0.1
        path: /health
        port: 2381
        scheme: HTTP
