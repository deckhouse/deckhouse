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
  initContainers:
    - name: init-chown
      image: {{ include "helm_lib_module_common_image" (list . "init") }}
      imagePullPolicy: Always
      command: ['sh', '-c', 'chown -R 64530:64530 /var/lib/etcd; chown -R 64530:64530 /etc/kubernetes/pki/etcd/']
      securityContext:
        runAsUser: 0
        runAsNonRoot: false
      volumeMounts:
      - mountPath: /var/lib/etcd
        name: etcd-data
      - mountPath: /etc/kubernetes/pki/etcd
        name: etcd-certs
  containers:
    - name: etcd
      securityContext:
        runAsUser: 64530
        runAsGroup: 64530
