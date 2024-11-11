apiVersion: v1
kind: Pod
metadata:
  labels:
    app: system-registry
    component: system-registry
    tier: control-plane
  annotations:
    authConfigHash: {{ quote .ConfigHashes.AuthTemplateHash }}
    distributionConfigHash: {{ quote .ConfigHashes.DistributionTemplateHash }}
    caCertHash: {{ quote .ConfigHashes.CaCertHash }}
    authCertHash: {{ quote .ConfigHashes.AuthCertHash }}
    authKeyHash: {{ quote .ConfigHashes.AuthKeyHash }}
    authTokenCertHash: {{ quote .ConfigHashes.AuthTokenCertHash }}
    authTokenKeyHash: {{ quote .ConfigHashes.AuthTokenKeyHash }}
    distributionCertHash: {{ quote .ConfigHashes.DistributionCertHash }}
    distributionKeyHash: {{ quote .ConfigHashes.DistributionKeyHash }}
  name: system-registry
  namespace: d8-system
spec:
  dnsPolicy: ClusterFirst
  hostNetwork: true
  containers:
  - name: distribution
    image: {{ .Images.DockerDistribution }}
    imagePullPolicy: IfNotPresent
    args:
      - serve
      - /config/config.yaml
{{- if .Proxy }}
    env:
      - name: HTTP_PROXY
        value: {{ .Proxy.HttpProxy }}
      - name: http_proxy
        value: {{ .Proxy.HttpProxy }}
      - name: HTTPS_PROXY
        value: {{ .Proxy.HttpsProxy }}
      - name: https_proxy
        value: {{ .Proxy.HttpsProxy }}
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
    image: {{ .Images.DockerAuth }}
    imagePullPolicy: IfNotPresent
    args:
      - -logtostderr
      - /config/config.yaml
    volumeMounts:
      - mountPath: /config
        name: auth-config-volume
      - mountPath: /system_registry_pki
        name: system-registry-pki-volume
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
  # Data volume
  - name: distribution-data-volume
    hostPath:
      path: /opt/deckhouse/system-registry/local_data
      type: DirectoryOrCreate
