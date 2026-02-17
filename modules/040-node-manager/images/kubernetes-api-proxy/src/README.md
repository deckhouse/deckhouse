# Kubernetes API Server Load Balancer

A lightweight TCP load balancer for Kubernetes API servers, 
designed to run as a StaticPod (on every machines in cluster) or as DaemonSet. 
It proxies raw TCP traffic to healthy upstream `kube-apiserver` 
instances and exposes simple HTTP health endpoints.

## Overview

This load balancer provides a reliable way to distribute traffic to Kubernetes API servers with built-in health checking, failover, and discovery capabilities. It's designed to be lightweight and efficient, using raw TCP proxying without TLS termination.

- **Pure TCP data path** using `tcpproxy` (no TLS termination in the proxy)
- **Health-aware upstream selection** based on `/readyz` checks
- **Latency tiering and scoring** to bias selection toward better performing servers
- **Periodic health checks with jitter** to avoid thundering herd
- **Dynamic upstream discovery** from Kubernetes `Endpoints` object (required)
- **TLS/mTLS and bearer token support** for upstream health checks (control path)
- **Kubernetes-friendly JSON logging** and health endpoints (`/healthz`, `/readyz`)

## Architecture

### Data Path
The `loadbalancer` package wraps `tcpproxy` and picks upstream records from `upstream.List`. It then proxies bytes between client and selected upstream.

### Control Path
The `upstream.List` periodically probes each upstream's `https://<host:port>/readyz` using an `http.Client` configured with optional TLS and Authorization headers. Probes assign a latency-based tier and adjust a score for success/failure. Selection prefers lower latency and non-negative score, with round-robin within a tier.

### Discovery
A background loop reads a Kubernetes `Endpoints` object and reconciles the set of upstream addresses.

## Getting Started

### Building

```bash
go build ./cmd/kubernetes-api-proxy
```

### Running Locally

```bash
go run ./cmd/kubernetes-api-proxy \
  --listen-addr=0.0.0.0 \
  --listen-port=7443 \
  --health-listen=:8080 \
  --log-level=info
```

## Health Endpoints

- `GET /healthz` → always `200 OK`
- `GET /readyz` → always `200 OK` ('cause in pressure situations it will try to balance traffic into default Kubernetes Service)
- `GET /upstreams` → returns current statics of balancing upstreams (for debug purposes)

## CLI Flags and Environment Variables

### Core Configuration
- `--listen-addr` (env: `LISTEN_ADDRESS`, default `0.0.0.0`)
- `--listen-port` (env: `LISTEN_PORT`, default `7443`)
- `--health-listen` (env: `HEALTH_LISTEN`, default `:8080`)
- `--log-level` (env: `LOG_LEVEL`, default `info`) — `debug|info|warn|error`

### Connection Settings
- `--dial-timeout` (default `5s`)
- `--keepalive-period` (default `1s`)
- `--tcp-user-timeout` (default `5s`)

### Health Check Settings
- `--health-interval` (default `1s`)
- `--health-timeout` (default `100ms`)
- `--health-jitter` (default `0.2`)

### Discovery Settings
- `--discover-period` (default `5s`) controls refresh interval
- The proxy automatically discovers upstream API servers from EndpointSlices of the `default/kubernetes` service
- It prefers port named `https`, then the first defined port, and defaults to `6443` if none

### Fallback Settings
- `--fallback-file` — Path to JSON file containing fallback upstreams (strings array)
- `--fallback-upstreams` — Comma-separated list of fallback upstreams (host:port)

## Kubernetes Deployment

This application can only be deployed as a DaemonSet or Static Pod.

### DaemonSet Deployment

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: kubernetes-api-proxy
  namespace: kube-system
  labels:
    app: apiserver-proxy-lb
spec:
  selector:
    matchLabels:
      app: kubernetes-api-proxy
  updateStrategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
  template:
    metadata:
      labels:
        app: kubernetes-api-proxy
    spec:
      serviceAccountName: kubernetes-api-proxy
      priorityClassName: system-cluster-critical
      hostNetwork: true
      dnsPolicy: ClusterFirstWithHostNet
      tolerations:
        - key: node-role.kubernetes.io/control-plane
          operator: Exists
          effect: NoSchedule
        - key: node-role.kubernetes.io/master
          operator: Exists
          effect: NoSchedule
      containers:
        - name: apiserver-lb
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
          image: kubernetes-api-proxy:0.0.1
          imagePullPolicy: IfNotPresent
          args:
            - "--listen-addr=0.0.0.0"
            - "--listen-port=6445"
            - "--health-listen=:8080"
            - "--log-level=debug"
          ports:
            - name: https
              containerPort: 6445
              hostPort: 6445
              protocol: TCP
            - name: health
              containerPort: 8080
              hostPort: 8080
              protocol: TCP
          readinessProbe:
            httpGet:
              path: /readyz
              port: health
            initialDelaySeconds: 2
            periodSeconds: 5
          livenessProbe:
            httpGet:
              path: /healthz
              port: health
            initialDelaySeconds: 2
            periodSeconds: 10
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 500m
              memory: 256Mi
```

### Static Pod Deployment

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: kubernetes-api-proxy
  namespace: kube-system
spec:
  priorityClassName: system-node-critical
  priority: 2000001000
  hostNetwork: true
  dnsPolicy: ClusterFirstWithHostNet
  volumes:
    - name: ca-cert
      hostPath:
        path: /etc/kubernetes/pki/ca.crt
        type: File
    - name: cl-cert
      hostPath:
        path: /etc/kubernetes/kap/apl.crt
        type: File
    - name: cl-key
      hostPath:
        path: /etc/kubernetes/kap/apl.key
        type: File
    - name: upstreams
      hostPath:
        path: /etc/kubernetes/kap/upstreams.json
        type: FileOrCreate
  containers:
    - name: kubernetes-api-proxy
      securityContext:
        allowPrivilegeEscalation: false
        capabilities:
          drop:
            - ALL
        readOnlyRootFilesystem: true
        runAsGroup: 0
        runAsNonRoot: false
        runAsUser: 0
        seccompProfile:
          type: RuntimeDefault
      image: kubernetes-api-proxy:0.0.1
      imagePullPolicy: IfNotPresent
      args:
        - "--listen-address=0.0.0.0"
        - "--listen-port=6445"
        - "--health-listen=:6480"
        - "--log-level=debug"
        - "--as-static-pod=true"
        - "--fallback-file=/var/run/kubernetes.io/kap/upstreams.json"
      ports:
        - name: https
          containerPort: 6445
          hostPort: 6445
          protocol: TCP
        - name: health
          containerPort: 6480
          protocol: TCP
      readinessProbe:
        httpGet:
          path: /readyz
          port: health
        initialDelaySeconds: 2
        periodSeconds: 5
      livenessProbe:
        httpGet:
          path: /healthz
          port: health
        initialDelaySeconds: 2
        periodSeconds: 10
      resources:
        requests:
          cpu: 50m
          memory: 64Mi
        limits:
          cpu: 500m
          memory: 256Mi
      volumeMounts:
        - name: cl-cert
          mountPath: /var/run/kubernetes.io/kap/cl.crt
          readOnly: true
        - name: cl-key
          mountPath: /var/run/kubernetes.io/kap/cl.key
          readOnly: true
        - name: ca-cert
          mountPath: /var/run/kubernetes.io/kap/ca.crt
          readOnly: true
        - name: upstreams
          mountPath: /var/run/kubernetes.io/kap/upstreams.json
```

### Minimal RBAC for Discovery

This application needs only read access to EndpointSlices. Apply the provided RBAC and use the ServiceAccount when deploying.

```bash
kubectl apply -f deploy/rbac.yaml
```

Then deploy with that ServiceAccount:

```bash
kubectl apply -f deploy/daemonset.yaml
```

## Operational Notes

- Readiness reflects upstream health only; the proxy itself is always alive on `/healthz`
- Timeouts are intentionally aggressive to avoid hanging on degraded upstreams
- Jitter (`--health-jitter`) staggers health checks to reduce synchronized load
- TLS settings apply to health checks only; data path is raw TCP proxying

## Project Structure

```
.
├── cmd/                    # Command-line applications
│   └── kubernetes-api-proxy/  # Main application
├── internal/               # Internal packages
│   ├── apiserver/          # API server implementation
│   ├── app/                # Application utilities
│   ├── config/             # Configuration handling
│   ├── loadbalancer/       # Load balancer implementation
│   └── upstream/           # Upstream server handling
├── pkg/                    # Public packages
│   ├── kubernetes/         # Kubernetes client utilities
│   └── utils/              # Some utils for slices
├── deploy/                 # Deployment manifests
│   └── hack/               # Bash script for CR creation (with certificates)
├── Dockerfile              # Docker build configuration
└── README.md               # This file
```

## License

Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
