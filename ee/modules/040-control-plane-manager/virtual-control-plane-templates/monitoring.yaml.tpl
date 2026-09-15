# Applied only when the PodMonitor/ServiceMonitor CRDs exist in the parent cluster.
# insecureSkipVerify: kcm and kube-scheduler serve metrics under a self-signed cert made at start.
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: vcp-kube-apiserver-${VCP_NAME}
  namespace: ${NAMESPACE}
  labels:
    heritage: deckhouse
    prometheus: main
    control-plane.deckhouse.io/virtual-control-plane: ${VCP_NAME}
spec:
  selector:
    matchLabels:
      control-plane.deckhouse.io/virtual-control-plane: ${VCP_NAME}
  endpoints:
  - port: https
    scheme: https
    path: /metrics
    interval: 30s
    scrapeTimeout: 25s
    bearerTokenSecret: {name: ${METRICS_TOKEN_SECRET_NAME}, key: token}
    tlsConfig: {insecureSkipVerify: true}
    relabelings:
    - action: keep
      sourceLabels: [__meta_kubernetes_service_name]
      regex: ${KUBE_APISERVER_SERVICE_NAME}
    - action: replace
      targetLabel: vcp
      replacement: ${VCP_NAME}
    - action: replace
      targetLabel: job
      replacement: vcp-kube-apiserver
---
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: vcp-kube-controller-manager-${VCP_NAME}
  namespace: ${NAMESPACE}
  labels:
    heritage: deckhouse
    prometheus: main
    control-plane.deckhouse.io/virtual-control-plane: ${VCP_NAME}
spec:
  selector:
    matchLabels:
      app: kube-controller-manager
      control-plane.deckhouse.io/vcp: ${VCP_NAME}
  podMetricsEndpoints:
  - port: https-metrics
    scheme: https
    path: /metrics
    interval: 30s
    scrapeTimeout: 25s
    bearerTokenSecret: {name: ${METRICS_TOKEN_SECRET_NAME}, key: token}
    tlsConfig: {insecureSkipVerify: true}
    relabelings:
    - action: keep
      sourceLabels: [__meta_kubernetes_pod_ready]
      regex: "true"
    - action: replace
      targetLabel: vcp
      replacement: ${VCP_NAME}
    - action: replace
      targetLabel: job
      replacement: vcp-kube-controller-manager
---
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: vcp-kube-scheduler-${VCP_NAME}
  namespace: ${NAMESPACE}
  labels:
    heritage: deckhouse
    prometheus: main
    control-plane.deckhouse.io/virtual-control-plane: ${VCP_NAME}
spec:
  selector:
    matchLabels:
      app: kube-scheduler
      control-plane.deckhouse.io/vcp: ${VCP_NAME}
  podMetricsEndpoints:
  - port: https-metrics
    scheme: https
    path: /metrics
    interval: 30s
    scrapeTimeout: 25s
    bearerTokenSecret: {name: ${METRICS_TOKEN_SECRET_NAME}, key: token}
    tlsConfig: {insecureSkipVerify: true}
    relabelings:
    - action: keep
      sourceLabels: [__meta_kubernetes_pod_ready]
      regex: "true"
    - action: replace
      targetLabel: vcp
      replacement: ${VCP_NAME}
    - action: replace
      targetLabel: job
      replacement: vcp-kube-scheduler
---
# kine and konnectivity-server are apiserver sidecars: same selector, other ports, plain HTTP.
# kine latency is the only datastore signal - the apiserver probes exclude the etcd check.
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: vcp-kine-${VCP_NAME}
  namespace: ${NAMESPACE}
  labels:
    heritage: deckhouse
    prometheus: main
    control-plane.deckhouse.io/virtual-control-plane: ${VCP_NAME}
spec:
  selector:
    matchLabels:
      app: kube-apiserver
      control-plane.deckhouse.io/vcp: ${VCP_NAME}
  podMetricsEndpoints:
  - port: metrics
    scheme: http
    path: /metrics
    interval: 30s
    scrapeTimeout: 25s
    relabelings:
    - action: keep
      sourceLabels: [__meta_kubernetes_pod_ready]
      regex: "true"
    - action: replace
      targetLabel: vcp
      replacement: ${VCP_NAME}
    - action: replace
      targetLabel: job
      replacement: vcp-kine
---
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: vcp-konnectivity-${VCP_NAME}
  namespace: ${NAMESPACE}
  labels:
    heritage: deckhouse
    prometheus: main
    control-plane.deckhouse.io/virtual-control-plane: ${VCP_NAME}
spec:
  selector:
    matchLabels:
      app: kube-apiserver
      control-plane.deckhouse.io/vcp: ${VCP_NAME}
  podMetricsEndpoints:
  - port: metrics-konn
    scheme: http
    path: /metrics
    interval: 30s
    scrapeTimeout: 25s
    relabelings:
    - action: keep
      sourceLabels: [__meta_kubernetes_pod_ready]
      regex: "true"
    - action: replace
      targetLabel: vcp
      replacement: ${VCP_NAME}
    - action: replace
      targetLabel: job
      replacement: vcp-konnectivity-server
