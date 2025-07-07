apiVersion: v1
kind: Pod
metadata:
  labels:
    app.kubernetes.io/managed-by: registry-nodeservices
    heritage: deckhouse
    module: registry
    app: registry
    component: registry-service
    tier: control-plane
    type: node-services
  annotations:
    registry.deckhouse.io/config-hash: {{ quote .Hash }}
    registry.deckhouse.io/config-version: {{ quote .Version }}
  name: registry-nodeservices
  namespace: d8-system
spec:
  securityContext:
    runAsGroup: 0
    runAsNonRoot: false
    runAsUser: 0
    seccompProfile:
      type: RuntimeDefault
  dnsPolicy: ClusterFirst
  hostNetwork: true
  containers:
  - name: distribution
    image: {{ .Images.Distribution }}
    imagePullPolicy: IfNotPresent
    args:
      - serve
      - /config/config.yaml
{{- with .Proxy }}
    env:
      {{- if .HTTP }}
      - name: HTTP_PROXY
        value: {{ .HTTP }}
      - name: http_proxy
        value: {{ .HTTP }}
      {{- end }}
      {{- if .HTTPS }}
      - name: HTTPS_PROXY
        value: {{ .HTTPS }}
      - name: https_proxy
        value: {{ .HTTPS }}
      {{- end }}
      {{- if .NoProxy }}
      - name: NO_PROXY
        value: {{ .NoProxy }}
      - name: no_proxy
        value: {{ .NoProxy }}
      {{- end }}
{{- end }}
    ports:
      - name: distribution
        containerPort: 5001
        hostPort: 5001
      - name: debug
        containerPort: 5002
    livenessProbe:
      httpGet:
        path: /
        port: distribution
        scheme: HTTPS
        {{- /*
          # use default host == PodIP && HostIP, because hostNetwork
        */}}
    readinessProbe:
      httpGet:
        path: /
        port: distribution
        scheme: HTTPS
        {{- /*
          # use default host == PodIP && HostIP, because hostNetwork
        */}}
    volumeMounts:
      - mountPath: /data
        name: data
      - mountPath: /config
        name: distribution-config
      - mountPath: /pki
        name: pki
  - name: auth
    image: {{ .Images.Auth }}
    imagePullPolicy: IfNotPresent
    ports:
      - name: auth
        containerPort: 5051
    livenessProbe:
      httpGet:
        path: /
        port: auth
        scheme: HTTPS
        host: 127.0.0.1
        {{- /*
          # can use host: 127.0.0.1, because hostNetwork
          # https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/#http-probes
        */}}
    readinessProbe:
      httpGet:
        path: /
        port: auth
        scheme: HTTPS
        host: 127.0.0.1
        {{- /*
          # can use host: 127.0.0.1, because hostNetwork
          # https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/#http-probes
        */}}
    args:
      - -logtostderr
      - /config/config.yaml
    volumeMounts:
      - mountPath: /config
        name: auth-config
      - mountPath: /pki
        name: pki
  {{- if .HasMirrorer }}
  - name: mirrorer
    image: {{ .Images.Mirrorer }}
    imagePullPolicy: IfNotPresent
    args:
      - /config/config.yaml
    volumeMounts:
      - mountPath: /config
        name: mirrorer-config
      - mountPath: /pki
        name: pki
  {{- end }}
  priorityClassName: system-node-critical
  volumes:
  # PKI
  - name: pki
    hostPath:
      path: /etc/kubernetes/registry/pki
      type: Directory
  # Configuration
  - name: auth-config
    hostPath:
      path: /etc/kubernetes/registry/auth
      type: DirectoryOrCreate
  - name: distribution-config
    hostPath:
      path: /etc/kubernetes/registry/distribution
      type: DirectoryOrCreate
  {{- if .HasMirrorer }}
  - name: mirrorer-config
    hostPath:
      path: /etc/kubernetes/registry/mirrorer
      type: DirectoryOrCreate
  {{- end }}
  # Data
  - name: data
    hostPath:
      path: /opt/deckhouse/registry/local_data
      type: DirectoryOrCreate
