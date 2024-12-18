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
  annotations:
    authConfigHash: {{ quote .Hashes.AuthTemplate }}
    distributionConfigHash: {{ quote .Hashes.DistributionTemplate }}
    caCertHash: {{ quote .Hashes.CACert }}
    authCertHash: {{ quote .Hashes.AuthCert }}
    authKeyHash: {{ quote .Hashes.AuthKey }}
    authTokenCertHash: {{ quote .Hashes.TokenCert }}
    authTokenKeyHash: {{ quote .Hashes.TokenKey }}
    distributionCertHash: {{ quote .Hashes.DistributionCert }}
    distributionKeyHash: {{ quote .Hashes.DistributionKey }}
  name: system-registry
  namespace: d8-system
spec:
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
    args:
      - -logtostderr
      - /config/config.yaml
    volumeMounts:
      - mountPath: /config
        name: auth-config-volume
      - mountPath: /system_registry_pki
        name: system-registry-pki-volume
  - name: mirrorer
    image: {{ .Images.Mirrorer }}
    imagePullPolicy: IfNotPresent
    env:
      - name: HOST_IP
        valueFrom:
          fieldRef:
            fieldPath: status.hostIP
    volumeMounts:
      - mountPath: /config
        name: mirrorer-config-volume
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
  - name: mirrorer-config-volume
    hostPath:
      path: /etc/kubernetes/system-registry/mirrorer
      type: DirectoryOrCreate
  # Data volume
  - name: distribution-data-volume
    hostPath:
      path: /opt/deckhouse/system-registry/local_data
      type: DirectoryOrCreate
