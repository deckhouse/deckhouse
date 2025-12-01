---
apiVersion: v1
kind: Pod
metadata:
  name: etcd
  namespace: kube-system
{{- if hasKey $ "images" }}
  {{- if hasKey $.images "controlPlaneManager" }}
    {{- if hasKey $.images.controlPlaneManager "etcd" }}
---
apiVersion: v1
kind: Pod
metadata:
  name: etcd
  namespace: kube-system
spec:
  containers:
    - name: etcd
      image: {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.controlPlaneManager "etcd") }}
    {{- end }}
  {{- end }}
{{- end }}
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
      startupProbe:
        failureThreshold: 24
        httpGet:
          host: 127.0.0.1
          path: /readyz?exclude=non_learner
          port: 2381
          scheme: HTTP
{{- $millicpu := $.resourcesRequestsMilliCpuControlPlane | default 512 -}}
{{- $memory := $.resourcesRequestsMemoryControlPlane | default 536870912 }}
---
apiVersion: v1
kind: Pod
metadata:
  name: etcd
  namespace: kube-system
spec:
  containers:
    - name: etcd
      resources:
        requests:
          cpu: "{{ div (mul $millicpu 35) 100 }}m"
          memory: "{{ div (mul $memory 35) 100 }}"
---
apiVersion: v1
kind: Pod
metadata:
  name: etcd
  namespace: kube-system
spec:
  dnsPolicy: ClusterFirstWithHostNet
---
apiVersion: v1
kind: Pod
metadata:
  name: etcd
  namespace: kube-system
spec:
  containers:
    - name: etcd
      securityContext:
        runAsNonRoot: false
        runAsUser: 0
        runAsGroup: 0
        capabilities:
          drop:
          - ALL
        readOnlyRootFilesystem: true
        seccompProfile:
          type: RuntimeDefault