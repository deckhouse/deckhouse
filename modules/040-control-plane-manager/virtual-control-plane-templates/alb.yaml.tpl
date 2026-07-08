apiVersion: network.deckhouse.io/v1alpha1
kind: ALBInstance
metadata:
  name: ${VCP_NAME}
  namespace: ${NAMESPACE}
  labels:
    heritage: deckhouse
    control-plane.deckhouse.io/vcp: ${VCP_NAME}
spec:
  gatewayName: ${VCP_NAME}
  inlet:
    type: LoadBalancer
    # TCP (not TLS passthrough) is required because in-cluster clients reach the apiserver via the ClusterIP by IP, sending no SNI.
    additionalPorts:
    - port: 6443
      protocol: TCP
---
# EgressSelectorConfiguration for the apiserver: route "cluster" traffic (logs/exec/metrics) to the
# konnectivity-server sidecar over the shared UDS.
apiVersion: v1
kind: ConfigMap
metadata:
  name: konnectivity-egress
  namespace: ${NAMESPACE}
  labels:
    heritage: deckhouse
    control-plane.deckhouse.io/vcp: ${VCP_NAME}
data:
  egress-selector-configuration.yaml: |
    apiVersion: apiserver.k8s.io/v1beta1
    kind: EgressSelectorConfiguration
    egressSelections:
    - name: cluster
      connection:
        proxyProtocol: GRPC
        transport:
          uds:
            udsName: /etc/kubernetes/konnectivity-server/konnectivity-server.socket
---
# Backend Service for the konnectivity-server kube-apiserver sidecar.
apiVersion: v1
kind: Service
metadata:
  name: konnectivity-server
  namespace: ${NAMESPACE}
  labels:
    heritage: deckhouse
    control-plane.deckhouse.io/vcp: ${VCP_NAME}
spec:
  type: ClusterIP
  selector:
    app: kube-apiserver
  ports:
  - name: agent
    port: 8132
    targetPort: 8132
    protocol: TCP
---
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: ${VCP_NAME}
  namespace: ${NAMESPACE}
  labels:
    heritage: deckhouse
    control-plane.deckhouse.io/vcp: ${VCP_NAME}
spec:
  parentRef:
    name: ${VCP_NAME}
    namespace: ${NAMESPACE}
  listeners:
  - name: konn
    port: 443
    protocol: TLS
    hostname: ${VCP_KONN_HOST}
    tls:
      mode: Passthrough
  - name: pkg
    port: 443
    protocol: TLS
    hostname: ${VCP_PKG_HOST}
    tls:
      mode: Passthrough
---
# Backend Service for RPP's tokenless bootstrap port (raw rpp-get binary)
apiVersion: v1
kind: Service
metadata:
  name: registry-packages-proxy-bootstrap
  namespace: d8-cloud-instance-manager
  labels:
    heritage: deckhouse
    control-plane.deckhouse.io/vcp: ${VCP_NAME}
spec:
  type: ClusterIP
  selector:
    app: registry-packages-proxy
  ports:
  - name: bootstrap
    port: 4282
    targetPort: 4282
    protocol: TCP
---
# Pure L4 route for the apiserver: matches by port only (no SNI), so it serves both external
# kubelet traffic (api.<vcp>:6443) and in-cluster clients that dial the ClusterIP by IP.
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: TCPRoute
metadata:
  name: ${VCP_NAME}-api
  namespace: ${NAMESPACE}
  labels:
    heritage: deckhouse
    control-plane.deckhouse.io/vcp: ${VCP_NAME}
spec:
  parentRefs:
  - name: ${VCP_NAME}
    namespace: ${NAMESPACE}
    sectionName: tcp-port-6443
    port: 6443
  rules:
  - backendRefs:
    - name: kube-apiserver
      port: 6443
---
apiVersion: gateway.networking.k8s.io/v1
kind: TLSRoute
metadata:
  name: ${VCP_NAME}-konn
  namespace: ${NAMESPACE}
  labels:
    heritage: deckhouse
    control-plane.deckhouse.io/vcp: ${VCP_NAME}
spec:
  parentRefs:
  - name: ${VCP_NAME}
    kind: ListenerSet
    group: gateway.networking.k8s.io
    sectionName: konn
    port: 443
  hostnames:
  - ${VCP_KONN_HOST}
  rules:
  - backendRefs:
    - name: konnectivity-server
      port: 8132
---
# SNI passthrough to RPP:443 (kube-rbac-proxy, token-gated). The token is the gate.
apiVersion: gateway.networking.k8s.io/v1
kind: TLSRoute
metadata:
  name: ${VCP_NAME}-packages
  namespace: ${NAMESPACE}
  labels:
    heritage: deckhouse
    control-plane.deckhouse.io/vcp: ${VCP_NAME}
spec:
  parentRefs:
  - name: ${VCP_NAME}
    kind: ListenerSet
    group: gateway.networking.k8s.io
    sectionName: pkg
    port: 443
  hostnames:
  - ${VCP_PKG_HOST}
  rules:
  - backendRefs:
    - name: registry-packages-proxy
      namespace: d8-cloud-instance-manager
      port: 443
---
# Plaintext bootstrap route: minget fetches the raw rpp-get binary here (tokenless), integrity via digest.
# Attaches directly to the Gateway built-in default HTTP:80 listener (d8-http-default)
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: ${VCP_NAME}-packages-bootstrap
  namespace: ${NAMESPACE}
  labels:
    heritage: deckhouse
    control-plane.deckhouse.io/vcp: ${VCP_NAME}
spec:
  parentRefs:
  - name: ${VCP_NAME}
    kind: Gateway
    group: gateway.networking.k8s.io
    sectionName: d8-http-default
    port: 80
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /
    backendRefs:
    - name: registry-packages-proxy-bootstrap
      namespace: d8-cloud-instance-manager
      port: 4282
---
# Permits the packages HTTPRoutes and TLSRoute (in the VCP namespace) to target the parent RPP Services.
apiVersion: gateway.networking.k8s.io/v1beta1
kind: ReferenceGrant
metadata:
  name: vcp-${VCP_NAME}-packages
  namespace: d8-cloud-instance-manager
  labels:
    heritage: deckhouse
    control-plane.deckhouse.io/vcp: ${VCP_NAME}
spec:
  from:
  - group: gateway.networking.k8s.io
    kind: HTTPRoute
    namespace: ${NAMESPACE}
  - group: gateway.networking.k8s.io
    kind: TLSRoute
    namespace: ${NAMESPACE}
  to:
  - group: ""
    kind: Service
    name: registry-packages-proxy
  - group: ""
    kind: Service
    name: registry-packages-proxy-bootstrap
