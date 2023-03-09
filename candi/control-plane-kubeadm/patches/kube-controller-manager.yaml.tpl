---
apiVersion: v1
kind: Pod
metadata:
  name: kube-controller-manager
  namespace: kube-system
  annotations:
    control-plane-manager.deckhouse.io/kubernetes-version: {{ $.clusterConfiguration.kubernetesVersion | quote }}
{{- if hasKey $ "images" }}
  {{- if hasKey $.images "controlPlaneManager" }}
    {{- $imageWithVersion := printf "kubeControllerManager%s" ($.clusterConfiguration.kubernetesVersion | replace "." "") }}
    {{- if hasKey $.images.controlPlaneManager $imageWithVersion }}
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-controller-manager
  namespace: kube-system
spec:
  containers:
    - name: kube-controller-manager
      image: {{ printf "%s%s:%s" $.registry.address $.registry.path (index $.images.controlPlaneManager $imageWithVersion) }}
    {{- end }}
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
          host: 127.0.0.1
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
          host: 127.0.0.1
          port: 10257
          scheme: HTTPS
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
# TODO remove after Docker support is dropped
  securityContext:
    seccompProfile:
      type: Unconfined
