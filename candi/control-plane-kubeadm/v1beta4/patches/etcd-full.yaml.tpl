{{- $etcdName := .nodeName | default "etcd-member" -}}
{{- $nodeIP := .nodeIP | default "127.0.0.1" -}}
{{- $advertiseClient := printf "https://%s:2379" $nodeIP -}}
{{- $listenPeer := printf "https://%s:2380" $nodeIP -}}
{{- $initialAdvertisePeer := printf "https://%s:2380" $nodeIP -}}
{{- $listenClient := printf "https://127.0.0.1:2379,https://%s:2379" $nodeIP -}}
{{- $initialCluster := printf "%s=%s" $etcdName $initialAdvertisePeer -}}
{{- $millicpu := .resourcesRequestsMilliCpuControlPlane | default 512 -}}
{{- $memory := .resourcesRequestsMemoryControlPlane | default 536870912 }}
---
apiVersion: v1
kind: Pod
metadata:
  name: etcd
  namespace: kube-system
  labels:
    component: etcd
    tier: control-plane
  annotations:
    control-plane-manager.deckhouse.io/etcd.advertise-client-urls: {{ $advertiseClient }}
spec:
  hostNetwork: true
  dnsPolicy: ClusterFirstWithHostNet
  priority: 2000001000
  priorityClassName: system-node-critical
  volumes:
  - name: etcd-data
    hostPath:
      path: /var/lib/etcd
      type: DirectoryOrCreate
  - name: etcd-certs
    hostPath:
      path: /etc/kubernetes/pki/etcd
      type: DirectoryOrCreate
  containers:
  - name: etcd
{{- if hasKey . "images" }}
{{- if hasKey .images "controlPlaneManager" }}
{{- if hasKey .images.controlPlaneManager "etcd" }}
    image: {{ printf "%s%s@%s" .registry.address .registry.path (index .images.controlPlaneManager "etcd") }}
    imagePullPolicy: IfNotPresent
{{- end }}
{{- end }}
{{- end }}
    command:
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
    - --feature-gates=InitialCorruptCheck=true
    - --watch-progress-notify-interval=5s
{{- if hasKey .etcd "quotaBackendBytes" }}
    - --quota-backend-bytes={{ .etcd.quotaBackendBytes | quote }}
    - --metrics=extensive
{{- end }}
{{- else }}
    - --initial-cluster-state=new
{{- end }}
{{- else }}
    - --initial-cluster-state=new
{{- end }}
{{- else }}
    - --initial-cluster-state=new
{{- end }}
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
    volumeMounts:
    - name: etcd-data
      mountPath: /var/lib/etcd
    - name: etcd-certs
      mountPath: /etc/kubernetes/pki/etcd
      readOnly: true
    resources:
      requests:
        cpu: "{{ div (mul $millicpu 35) 100 }}m"
        memory: "{{ div (mul $memory 35) 100 }}"
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
