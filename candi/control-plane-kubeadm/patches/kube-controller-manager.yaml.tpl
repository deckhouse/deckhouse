---
apiVersion: v1
kind: Pod
metadata:
  name: kube-controller-manager
  namespace: kube-system
  annotations:
    control-plane-manager.deckhouse.io/kubernetes-version: {{ $.clusterConfiguration.kubernetesVersion | quote }}
{{- if hasKey . "images" }}
  {{- if hasKey $.images  "kube-controller-manager" }}
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-controller-manager
  namespace: kube-system
spec:
  containers:
    - name: kube-controller-manager
      image: {{ pluck "kube-controller-manager" $.images | first }}
  {{- end }}
{{- end }}
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
  name: kube-controller-manager
  namespace: kube-system
spec:
  containers:
    - name: kube-controller-manager
      livenessProbe:
        httpGet:
          scheme: HTTPS
          port: 10257
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
{{- end }}
{{- $millicpu := $.resourcesRequestsMilliCpuControlPlane | default 512 -}}
{{- $memory := $.resourcesRequestsMemoryControlPlane | default 536870912 }}
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-controller-manager
  namespace: kube-system
spec:
  containers:
    - name: kube-controller-manager
      resources:
        requests:
          cpu: "{{ div (mul $millicpu 20) 100 }}m"
          memory: "{{ div (mul $memory 20) 100 }}"
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-controller-manager
  namespace: kube-system
spec:
  dnsPolicy: ClusterFirstWithHostNet
