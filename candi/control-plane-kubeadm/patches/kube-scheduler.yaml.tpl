---
apiVersion: v1
kind: Pod
metadata:
  name: kube-scheduler
  namespace: kube-system
  annotations:
    control-plane-manager.deckhouse.io/kubernetes-version: {{ $.clusterConfiguration.kubernetesVersion | quote }}
{{- if hasKey . "images" }}
  {{- if hasKey $.images "kube-scheduler" }}
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-scheduler
  namespace: kube-system
spec:
  containers:
    - name: kube-scheduler
      image: {{ pluck "kube-scheduler" $.images | first }}
  {{- end }}
{{- end }}
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
  name: kube-scheduler
  namespace: kube-system
spec:
  containers:
    - name: kube-scheduler
      livenessProbe:
        httpGet:
          scheme: HTTPS
          port: 10259
{{- if hasKey . "nodeIP" }}
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
{{- $millicpu := $.resourcesRequestsMilliCpuControlPlane | default 512 -}}
{{- $memory := $.resourcesRequestsMemoryControlPlane | default 536870912 }}
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-scheduler
  namespace: kube-system
spec:
  containers:
    - name: kube-scheduler
      resources:
        requests:
          cpu: "{{ div (mul $millicpu 10) 100 }}m"
          memory: "{{ div (mul $memory 10) 100 }}"
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-scheduler
  namespace: kube-system
spec:
  dnsPolicy: ClusterFirstWithHostNet
