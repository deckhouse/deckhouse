---
apiVersion: v1
kind: Pod
metadata:
  name: kube-scheduler
  namespace: kube-system
  annotations:
    control-plane-manager.deckhouse.io/kubernetes-version: {{ $.clusterConfiguration.kubernetesVersion | quote }}
{{- if hasKey $ "images" }}
  {{- if hasKey $.images "controlPlaneManager" }}
    {{- $imageWithVersion := printf "kubeScheduler%s" ($.clusterConfiguration.kubernetesVersion | replace "." "") }}
    {{- if hasKey $.images.controlPlaneManager $imageWithVersion }}
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-scheduler
  namespace: kube-system
spec:
  containers:
    - name: kube-scheduler
      image: {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.controlPlaneManager $imageWithVersion) }}
    {{- end }}
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
          host: 127.0.0.1
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
          host: 127.0.0.1
          port: 10259
          scheme: HTTPS
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
