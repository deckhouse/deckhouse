{{- $etcdName := .nodeName | default "etcd-member" -}}
{{- $nodeIP := .nodeIP | default "127.0.0.1" -}}
{{- $advertiseClient := printf "https://%s:2379" $nodeIP -}}
{{- $listenPeer := printf "https://%s:2380" $nodeIP -}}
{{- $initialAdvertisePeer := printf "https://%s:2380" $nodeIP -}}
{{- $listenClient := printf "https://127.0.0.1:2379,https://%s:2379" $nodeIP -}}
{{- $initialCluster := printf "%s=%s" $etcdName $initialAdvertisePeer -}}
{{- $resourcesRequests := dict -}}
{{- if and .settings .settings.resourcesRequests -}}
  {{- $resourcesRequests = .settings.resourcesRequests -}}
{{- end -}}
{{- $nodesCount := .nodesCount | default 0 | int -}}
{{- $maxMilliCPU := $resourcesRequests.maxMilliCPU | default 0 | int -}}
{{- $maxMemoryBytes := $resourcesRequests.maxMemoryBytes | default 0 | int -}}
{{- /*
  Resource requests for the etcd static pod (component share: 35%).

  Manual override (controlPlaneManager.resourcesRequests) arrives as a single
  pool and is split by the historical component share (CPU and memory
  independently). Otherwise requests are sized per-component in discrete tiers
  by the cluster node count — stepped, not linear, so the static pod stays
  stable within a tier and only changes at rare tier boundaries. The auto value
  is clamped to its share of the node safety cap ($maxMilliCPU / $maxMemoryBytes)
  computed by the hook, so it never crowds out other pods on an undersized master.
*/ -}}
{{- $millicpu := 0 -}}
{{- $memory := 0 -}}
{{- if $resourcesRequests.milliCPU -}}
  {{- $millicpu = div (mul $resourcesRequests.milliCPU 35) 100 -}}
{{- else -}}
  {{- if lt $nodesCount 25 -}}{{- $millicpu = 150 -}}
  {{- else if lt $nodesCount 100 -}}{{- $millicpu = 300 -}}
  {{- else if lt $nodesCount 250 -}}{{- $millicpu = 600 -}}
  {{- else if lt $nodesCount 500 -}}{{- $millicpu = 1100 -}}
  {{- else -}}{{- $millicpu = 1500 -}}
  {{- end -}}
  {{- if $maxMilliCPU -}}{{- $millicpu = min $millicpu (div (mul $maxMilliCPU 35) 100) -}}{{- end -}}
{{- end -}}
{{- if $resourcesRequests.memoryBytes -}}
  {{- $memory = div (mul $resourcesRequests.memoryBytes 35) 100 -}}
{{- else -}}
  {{- if lt $nodesCount 25 -}}{{- $memory = 768 -}}
  {{- else if lt $nodesCount 100 -}}{{- $memory = 1408 -}}
  {{- else if lt $nodesCount 250 -}}{{- $memory = 2560 -}}
  {{- else -}}{{- $memory = 4096 -}}
  {{- end -}}
  {{- $memory = mul $memory 1048576 -}}
  {{- if $maxMemoryBytes -}}{{- $memory = min $memory (div (mul $maxMemoryBytes 35) 100) -}}{{- end -}}
{{- end }}
{{- /* etcd */ -}}
apiVersion: v1
kind: Pod
metadata:
  annotations:
    control-plane-manager.deckhouse.io/etcd.advertise-client-urls: {{ $advertiseClient }}
  labels:
    component: etcd
    tier: control-plane
  name: etcd
  namespace: kube-system
spec:
  containers:
  - command:
    - etcd
    - --advertise-client-urls={{ $advertiseClient }}
    - --cert-file=/etc/kubernetes/pki/etcd/server.crt
    - --client-cert-auth=true
    - --data-dir=/var/lib/etcd
    - --initial-advertise-peer-urls={{ $initialAdvertisePeer }}
    - --initial-cluster={{ $initialCluster }}
    {{- if hasKey . "etcd" }}
    {{- if hasKey .etcd "existingCluster" }}
    {{- if .etcd.existingCluster }}
    - --initial-cluster-state=existing
    {{- end }}
    {{- end }}
    {{- end }}
    {{- if semverCompare "< 1.34" .clusterConfiguration.kubernetesVersion }}
    - --feature-gates=InitialCorruptCheck=true
    {{- end }}
    - --watch-progress-notify-interval=5s
    - --quota-backend-bytes={{ (.etcd).quotaBackendBytes | default 2147483648 }}
    - --metrics=extensive
    - --key-file=/etc/kubernetes/pki/etcd/server.key
    - --listen-client-urls={{ $listenClient }}
    - --listen-metrics-urls=http://127.0.0.1:2381
    - --listen-peer-urls={{ $listenPeer }}
    - --name={{ $etcdName }}
    - --peer-cert-file=/etc/kubernetes/pki/etcd/peer.crt
    - --peer-client-cert-auth=true
    - --peer-key-file=/etc/kubernetes/pki/etcd/peer.key
    - --peer-trusted-ca-file=/etc/kubernetes/pki/etcd/ca.crt
    - --snapshot-count=10000
    - --trusted-ca-file=/etc/kubernetes/pki/etcd/ca.crt
    env:
    - name: ETCD_HEARTBEAT_INTERVAL
      value: "100"
    - name: ETCD_ELECTION_TIMEOUT
      value: "1000"
  {{- if ((.images).controlPlaneManager).etcd }}  
    image: {{ printf "%s%s@%s" .registry.address .registry.path (index .images.controlPlaneManager "etcd") }}
    imagePullPolicy: IfNotPresent
  {{- end }}
    livenessProbe:
      failureThreshold: 8
      httpGet:
        host: 127.0.0.1
        path: /livez
        port: probe-port
        scheme: HTTP
      initialDelaySeconds: 10
      periodSeconds: 10
      timeoutSeconds: 15
    name: etcd
    ports:
    - containerPort: 2381
      name: probe-port
      protocol: TCP
    readinessProbe:
      failureThreshold: 3
      httpGet:
        host: 127.0.0.1
        path: /health
        port: 2381
        scheme: HTTP
      periodSeconds: 1
      timeoutSeconds: 15
    resources:
      requests:
        cpu: "{{ $millicpu }}m"
        memory: "{{ $memory }}"
    securityContext:
      capabilities:
        drop:
        - ALL
      readOnlyRootFilesystem: true
      runAsGroup: 0
      runAsNonRoot: false
      runAsUser: 0
      seccompProfile:
        type: RuntimeDefault
    startupProbe:
      failureThreshold: 24
      httpGet:
        host: 127.0.0.1
        path: /readyz?exclude=non_learner
        port: 2381
        scheme: HTTP
      initialDelaySeconds: 10
      periodSeconds: 10
      timeoutSeconds: 15
    volumeMounts:
    - mountPath: /var/lib/etcd
      name: etcd-data
    - mountPath: /etc/kubernetes/pki/etcd
      name: etcd-certs
      readOnly: true
  dnsPolicy: ClusterFirstWithHostNet
  hostNetwork: true
  priority: 2000001000
  priorityClassName: system-node-critical
  securityContext:
    seccompProfile:
      type: RuntimeDefault
  volumes:
  - hostPath:
      path: /var/lib/etcd
      type: DirectoryOrCreate
    name: etcd-data
  - hostPath:
      path: /etc/kubernetes/pki/etcd
      type: DirectoryOrCreate
    name: etcd-certs
