
apiVersion: v1
kind: Pod
metadata:
  labels:
    component: system-registry
    tier: control-plane
  name: system-registry
  namespace: d8-system
spec:
  dnsPolicy: ClusterFirst
  hostNetwork: true
  containers:
  - name: distribution
    image: "{{ .images.systemRegistry.dockerDistribution }}"
    imagePullPolicy: IfNotPresent
    args:
      - serve
      - /config/config.yaml
    volumeMounts:
      - mountPath: /config
        name: distribution-config-volume
      - mountPath: /system_registry_pki
        name: system-registry-pki-volume
  - name: auth
    image: "{{ .images.systemRegistry.dockerAuth }}"
    imagePullPolicy: IfNotPresent
    args:
      - -logtostderr
      - /config/config.yaml
    volumeMounts:
      - mountPath: /config
        name: auth-config-volume
      - mountPath: /system_registry_pki
        name: system-registry-pki-volume
  - name: seaweedfs
    image: "{{ .images.systemRegistry.seaweedfs }}"
    imagePullPolicy: IfNotPresent
    args:
      - -config_dir=/config
      - -logtostderr=true
      - -v=0
      - server
      - -filer
      - -s3
      - -dir=/data
      - -volume.port=8081
      - -volume.max=0
      - -master.volumeSizeLimitMB=1024
      - -master.raftHashicorp
      {{- if .isRaftBootstrap }}
      - -master.raftBootstrap
      {{- end }}
      - -metricsPort=9324
      - -volume.readMode=redirect
      - -s3.allowDeleteBucketNotEmpty=true
      - -master.defaultReplication=000
      - -volume.pprof
      - -filer.maxMB=16
      - -ip={{ .hostIP }}
      {{- if eq (len .masterPeers) 0 }}
      - -master.peers={{ .hostIP }}:9333
      {{- else }}
      - -master.peers={{ range $index, $masterPeerAddr := .masterPeers }}{{ if $index }},{{ end }}{{ printf "%s:%s" $masterPeerAddr "9333" }}{{ end }}
      {{- end }}
    env:
      - name: GOGC
        value: "20"
      - name: GOMEMLIMIT
        value: "500MiB"
    volumeMounts:
      - mountPath: /data
        name: seaweedfs-data-volume
      - mountPath: /config
        name: seaweedfs-config-volume
      - mountPath: /kubernetes_pki
        name: kubernetes-pki-volume
      - mountPath: /system_registry_pki
        name: system-registry-pki-volume
  priorityClassName: system-node-critical
  volumes:
  # PKI
  - name: kubernetes-pki-volume
    hostPath:
      path: /etc/kubernetes/pki
      type: Directory
  - name: system-registry-pki-volume
    hostPath:
      path: /etc/kubernetes/system-registry/pki
      type: Directory
  # Configs
  - name: auth-config-volume
    hostPath:
      path: /etc/kubernetes/system-registry/auth_config
      type: DirectoryOrCreate
  - name: seaweedfs-config-volume
    hostPath:
      path: /etc/kubernetes/system-registry/seaweedfs_config
      type: DirectoryOrCreate
  - name: distribution-config-volume
    hostPath:
      path: /etc/kubernetes/system-registry/distribution_config
      type: DirectoryOrCreate
  # Data
  - name: seaweedfs-data-volume
    hostPath:
      path: /opt/deckhouse/system-registry/seaweedfs_data
      type: DirectoryOrCreate
  # - name: tmp
  #   emptyDir: {}
