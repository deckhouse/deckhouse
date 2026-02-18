---
apiVersion: v1
kind: Pod
metadata:
  name: kube-apiserver
  namespace: kube-system
  annotations:
    control-plane-manager.deckhouse.io/kubernetes-version: {{ $.clusterConfiguration.kubernetesVersion | quote }}
{{- if hasKey $ "images" }}
  {{- if hasKey $.images "controlPlaneManager" }}
    {{- $imageWithVersion := printf "kubeApiserver%s" ($.clusterConfiguration.kubernetesVersion | replace "." "") }}
    {{- if hasKey $.images.controlPlaneManager $imageWithVersion }}
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-apiserver
  namespace: kube-system
spec:
  containers:
    - name: kube-apiserver
      image: {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.controlPlaneManager $imageWithVersion) }}
    {{- end }}
  {{- end }}
{{- end }}
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
          cpu: "{{ div (mul $millicpu 33) 100 }}m"
          memory: "{{ div (mul $memory 33) 100 }}"
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

{{- if hasKey $ "images" }}
  {{- if hasKey $.images "controlPlaneManager" }}
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-apiserver
  namespace: kube-system
spec:
  containers:
  - name: kube-apiserver
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
    readinessProbe:
      httpGet:
    {{- if hasKey . "nodeIP" }}
        host: {{ .nodeIP | quote }}
    {{- end }}
        path: /healthz
        port: 6443
        scheme: HTTPS
    livenessProbe:
      httpGet:
    {{- if hasKey . "nodeIP" }}
        host: {{ .nodeIP | quote }}
    {{- end }}
        path: /livez
        port: 6443
        scheme: HTTPS
    startupProbe:
      httpGet:
    {{- if hasKey . "nodeIP" }}
        host: {{ .nodeIP | quote }}
    {{- end }}
        path: /livez
        port: 6443
        scheme: HTTPS
    env:
    - name: GOGC
      value: "50"
  {{- end }}
{{- end }}

{{- if .apiserver.serviceAccount }}
  {{- if .apiserver.serviceAccount.additionalAPIIssuers }}
    {{- $defaultIssuer := printf "https://kubernetes.default.svc.%s" .clusterConfiguration.clusterDomain }}
    {{- $issuerToRemove := default $defaultIssuer .apiserver.serviceAccount.issuer }}
    {{- $uniqueIssuers := uniq .apiserver.serviceAccount.additionalAPIIssuers }}
    {{- if not (and (eq (len $uniqueIssuers) 1) (eq (index $uniqueIssuers 0) $issuerToRemove)) }}
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-apiserver
  namespace: kube-system
spec:
  containers:
  - name: kube-apiserver
    args:
    {{- range $uniqueIssuers }}
      {{- if ne . $issuerToRemove }}
    - --service-account-issuer={{ . }}
      {{- end }}
    {{- end }}
    {{- end }}
  {{- end }}
{{- end }}
