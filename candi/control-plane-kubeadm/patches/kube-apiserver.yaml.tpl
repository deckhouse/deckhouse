---
apiVersion: v1
kind: Pod
metadata:
  name: kube-apiserver
  namespace: kube-system
  annotations:
    control-plane-manager.deckhouse.io/kubernetes-version: {{ $.clusterConfiguration.kubernetesVersion | quote }}
{{- if hasKey . "images" }}
  {{- if hasKey $.images "kube-apiserver" }}
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-apiserver
  namespace: kube-system
spec:
  containers:
    - name: kube-apiserver
      image: {{ pluck "kube-apiserver" $.images | first }}
  {{- end }}
{{- end }}
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
{{- $millicpu := $.resourcesRequestsMilliCpuControlPlane | default 512 -}}
{{- $memory := $.resourcesRequestsMemoryControlPlane | default 536870912 }}
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-apiserver
  namespace: kube-system
spec:
  containers:
    - name: kube-apiserver
      resources:
        requests:
          cpu: "{{ div (mul $millicpu 35) 100 }}m"
          memory: "{{ div (mul $memory 35) 100 }}"
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-apiserver
  namespace: kube-system
spec:
  dnsPolicy: ClusterFirstWithHostNet
{{- if $.apiserver.oidcIssuerAddress }}
  {{- if $.apiserver.oidcIssuerURL }}
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-apiserver
  namespace: kube-system
spec:
  hostAliases:
  - ip: {{ $.apiserver.oidcIssuerAddress }}
    hostnames:
    - {{ trimSuffix "/" (trimPrefix "https://" $.apiserver.oidcIssuerURL) }}
  {{- end }}
{{- end }}
