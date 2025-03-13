apiVersion: v1
kind: Pod
metadata:
  labels:
    app.kubernetes.io/managed-by: system-registry
    heritage: deckhouse
    module: system-registry
    app: system-registry
    component: system-registry
    tier: control-plane
    type: static-pod
  annotations:
    registry.deckhouse.io/auth-config-hash: {{ quote .Hashes.AuthTemplate }}
    registry.deckhouse.io/distribution-config-hash: {{ quote .Hashes.DistributionTemplate }}
    {{- if eq .Registry.Mode "Detached" }}
    registry.deckhouse.io/mirrorer-config-hash: {{ quote .Hashes.MirrorerTemplate }}
    {{- end }}
    registry.deckhouse.io/ca-cert-hash: {{ quote .Hashes.CACert }}
    registry.deckhouse.io/auth-cert-hash: {{ quote .Hashes.AuthCert }}
    registry.deckhouse.io/auth-key-hash: {{ quote .Hashes.AuthKey }}
    registry.deckhouse.io/auth-token-cert-hash: {{ quote .Hashes.TokenCert }}
    registry.deckhouse.io/auth-token-key-hash: {{ quote .Hashes.TokenKey }}
    registry.deckhouse.io/distribution-cert-hash: {{ quote .Hashes.DistributionCert }}
    registry.deckhouse.io/distribution-key-hash: {{ quote .Hashes.DistributionKey }}
    registry.deckhouse.io/ingress-client-ca-cert-hash: {{ quote .Hashes.IngressClientCACert }}
    registry.deckhouse.io/upstream-registry-ca-cert-hash: {{ quote .Hashes.UpstreamRegistryCACert }}
    {{- if .Version }}
    registry.deckhouse.io/config-version: {{ quote .Version }}
    {{- else }}
    registry.deckhouse.io/config-version: "unknown"
    {{- end }}
  name: system-registry
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
{{- if .Proxy }}
    env:
      - name: HTTP_PROXY
        value: {{ .Proxy.Http }}
      - name: http_proxy
        value: {{ .Proxy.Http }}
      - name: HTTPS_PROXY
        value: {{ .Proxy.Https }}
      - name: https_proxy
        value: {{ .Proxy.Https }}
      - name: NO_PROXY
        value: {{ .Proxy.NoProxy }}
      - name: no_proxy
        value: {{ .Proxy.NoProxy }}
{{- end }}
    ports:
      - name: emb-reg-dist
        containerPort: 5001
        hostPort: 5001
      - name: emb-reg-debug
        containerPort: 5002
    livenessProbe:
      httpGet:
        path: /
        port: emb-reg-dist
        scheme: HTTPS
        {{- /*
          # use default host == PodIP && HostIP, because hostNetwork
        */}}
    readinessProbe:
      httpGet:
        path: /
        port: emb-reg-dist
        scheme: HTTPS
        {{- /*
          # use default host == PodIP && HostIP, because hostNetwork
        */}}
    volumeMounts:
      - mountPath: /data
        name: distribution-data-volume
      - mountPath: /config
        name: distribution-config-volume
      - mountPath: /system_registry_pki
        name: system-registry-pki-volume
  - name: auth
    image: {{ .Images.Auth }}
    imagePullPolicy: IfNotPresent
    ports:
      - name: emb-reg-auth
        containerPort: 5051
    livenessProbe:
      httpGet:
        path: /
        port: emb-reg-auth
        scheme: HTTPS
        host: 127.0.0.1
        {{- /*
          # can use host: 127.0.0.1, because hostNetwork
          # https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/#http-probes
        */}}
    readinessProbe:
      httpGet:
        path: /
        port: emb-reg-auth
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
        name: auth-config-volume
      - mountPath: /system_registry_pki
        name: system-registry-pki-volume
  {{- if and (eq .Registry.Mode "Detached") (gt (len .Mirrorer.Upstreams) 0) }}
  - name: mirrorer
    image: {{ .Images.Mirrorer }}
    imagePullPolicy: IfNotPresent
    args:
      - /config/config.yaml
    volumeMounts:
      - mountPath: /config
        name: mirrorer-config-volume
      - mountPath: /system_registry_pki
        name: system-registry-pki-volume
  {{- end }}
  priorityClassName: system-node-critical
  volumes:
  # PKI volumes
  - name: kubernetes-pki-volume
    hostPath:
      path: /etc/kubernetes/pki
      type: Directory
  - name: system-registry-pki-volume
    hostPath:
      path: /etc/kubernetes/system-registry/pki
      type: Directory
  # Configuration volumes
  - name: auth-config-volume
    hostPath:
      path: /etc/kubernetes/system-registry/auth_config
      type: DirectoryOrCreate
  - name: distribution-config-volume
    hostPath:
      path: /etc/kubernetes/system-registry/distribution_config
      type: DirectoryOrCreate
  {{- if eq .Registry.Mode "Detached" }}
  - name: mirrorer-config-volume
    hostPath:
      path: /etc/kubernetes/system-registry/mirrorer
      type: DirectoryOrCreate
  {{- end }}
  # Data volume
  - name: distribution-data-volume
    hostPath:
      path: /opt/deckhouse/system-registry/local_data
      type: DirectoryOrCreate
