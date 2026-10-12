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

### Data path

The `loadbalancer` package wraps `tcpproxy`. For every client connection
it picks an upstream (the main `upstream.List` first, then the fallback
list, then `KUBERNETES_SERVICE_HOST:KUBERNETES_SERVICE_PORT`), dials it
with `--dial-timeout` (`0` means 10s) and copies bytes in both directions.

An upstream from the main list gets one retry on the next best upstream
(`PickNot`) in two cases:

- The dial fails. The upstream is marked down (`Down`) and the connection
  is dialed to the next one. If that dial fails too, the second upstream
  is marked down as well and the client connection is closed.
- The upstream accepts the connection but sends nothing within the
  first-byte budget after the client's first byte. This is what a
  livelocked or frozen apiserver looks like: the kernel still answers
  TCP, the process never replies, and without the retry the client waits
  out its own TLS handshake timeout (10s in client-go). The proxy keeps a
  copy of what the client sent so far (up to 64 KiB; a TLS client sends
  its ClientHello and then waits), marks the upstream silent (`Silent`),
  dials the next one, replays the copy and waits for the first byte
  again. A second silence marks that upstream silent too but is not
  retried: the connection stays on it and the client decides.

A connection switches upstream at most once, so a dial retry followed by
a silence stays on the second upstream. The wait starts with the
client's first byte, not with the dial: an idle client is not a silent
upstream. Only a timeout counts as silence: the first-byte deadline, or
the kernel giving up on the connection (`--tcp-user-timeout`). A close,
a reset or any other error is not. An upstream that accepts and then
closes before sending anything is not retried; the client sees the close
and retries itself. The proxy marks the upstream silent and leaves the
connection where it is when there is no alternative, when the
alternative refuses the dial (it is marked down too), or when the client
sent more than 64 KiB before the upstream answered and the copy was
dropped. The connection also stays, with the alternative closed and not
marked, when no time is left for it or when replaying the copy to it
fails.

The budget is kept per upstream, so a healthy but slow apiserver (a TLS
handshake takes 3-5s during a reconnect storm) is not abandoned: the
budget starts at `--first-byte-timeout`, becomes twice the observed
first-byte latency after an answer (clamped between the floor and twice
the floor), and grows by half after a timeout (capped at twice the
floor).

Every connection also has a chain budget of three floors (9s by
default) counted from the client's first byte. The alternative's dial
and first-byte wait are cut to whatever is left of it. A silence on a
wait that was cut short is inconclusive: the upstream is not marked and
the connection stays on it. Keep three times `--first-byte-timeout`
below the clients' TLS handshake timeout (10s in client-go), otherwise
the retry lands after the client has given up. `--first-byte-timeout=0`
turns the wait off; the dial retry stays.

Fallback and `KUBERNETES_SERVICE_HOST` upstreams get the plain path: dial,
close the client on a dial error, otherwise copy. They are never marked
down or retried, and their latency does not feed the budget.

### Control path

`upstream.List` probes every upstream once per `--health-interval` with
`GET /readyz` and a `--health-timeout` budget (500ms). Probes share one
keep-alive HTTPS transport from `rest.TransportFor` (HTTP/2 when the
apiserver offers it), so a probe normally travels over a warm connection.

A probe ends in one of four results: `ready` (200), `not-ready` (any other
status), `timeout` (no answer within the budget on an established
connection: the apiserver is slow, or hung) and `unreachable` (no
connection, or the connection failed with anything but a timeout).

Each round is judged as a whole. Two things are decided before the
results are applied.

Warming up. An apiserver that restarts goes `unreachable` first (the
listener closes, then reopens) and answers `not-ready` for the next
20-80 seconds while it warms up. A hung etcd never produces
`unreachable`. So an upstream whose `not-ready` follows an `unreachable`
(probe timeouts in between do not break the sequence), or a newly
discovered upstream whose first answer is `not-ready`, is marked `warm`
until its first 200. A warm upstream does not vote below,
is never restored by the nobody-ready rule, and every `not-ready` costs
it a point. Upstreams known since the proxy started are exempt: at that
moment everything is new and nothing can be inferred.

Nobody ready. When no upstream answered 200, the proxy counts how many of
the voting upstreams said `not-ready`. Voters are the upstreams that
answered `ready`, `not-ready` or `timeout`; `unreachable`, `warm` and
upstreams missing from the EndpointSlice do not vote. If `not-ready` is a
strict majority, the cause is shared (etcd, an authorization webhook) and
picking another upstream would not help: the status is ignored and the
latency sets the tier, so traffic goes to whoever answers fastest. A tie
is not a majority: one restarting master against one slow master is not
a shared cause. When nobody votes at all (every upstream `unreachable`,
`warm` or missing), the previous verdict stands.

With that settled:

- 200: the latency sets the tier and the score goes up by one.
- `not-ready` while someone answered 200, or while `not-ready` is not a
  majority, or on a warm upstream: `failed` tier, minus one.
- `not-ready` while nobody is ready and `not-ready` is a majority: the
  latency sets the tier, score unchanged.
- `timeout`: `failed` tier, score unchanged. A slow apiserver can still
  serve; a hung one is caught by the first-byte retry on the data path.
- `unreachable`: `failed` tier, minus one.

A dial error (`Down`) or a first-byte timeout (`Silent`) on the data path
also moves the upstream to `failed`, minus one, and holds it there for
`--dial-timeout`: probes ride a warm connection and cannot see that new
connections are refused or left unanswered. Every failed connection
costs a point, so a hung apiserver under a connection storm reaches the
score floor within a second and is picked only when nothing better is
left; once it answers 200 again, each probe gives a point back.

Tiers are latency buckets `[0, 10ms, 25ms, 50ms, 100ms, 250ms, 500ms]`,
numbered 0 to 6, plus `failed` (7). The score runs from -5 to 3. A new
upstream starts with score 1 in the `failed` tier.

Selection order: an upstream outside the `failed` tier first. Among
`failed` upstreams, the one the proxy knows least bad about wins: silent
(`timeout`, no result yet, or a held upstream whose last probe was 200
or an ignored `not-ready`) before `not-ready` before `warm` before
`unreachable`, `Down` or `Silent` (the last two only until the next
probe) before missing from the EndpointSlice. Then a non-negative score
beats a negative one; among negative `failed` upstreams the highest
score wins; among live upstreams the lowest tier wins, round-robin
inside it. The fallback list is consulted only when the main list is
empty. On start the main list is seeded from the fallback list before
the port opens, so connections accepted before the first discovery round
get the dial retry, the first-byte retry and the stall close too.

`/upstreams` shows for every upstream the last probe result
(`lastProbe`), the `warm` flag, whether a `Down` or `Silent` happened
since the last probe (`downed`), whether the hold is still running
(`held`), whether a stall probe is in flight (`stalling`) and how many
times its client connections were closed as stalled (`evictions`).

#### Closing connections to a stalled apiserver

kubelet, cilium, kube-proxy and the cloud provider manager keep one HTTP/2
connection to the apiserver open for hours; kube-controller-manager and
kube-scheduler will do the same once they go through the proxy. The
first-byte retry does not help them, because it acts only before the
upstream has answered anything. When the apiserver behind such a
connection hangs (the kernel still accepts TCP, the process answers
nothing), client-go notices only through its HTTP/2 health check: 30
seconds without a frame, then a PING that times out after 15 seconds. If
the handlers hang but PINGs are still answered, it never notices. In the
meantime kube-controller-manager loses its leader lease (10s renew
deadline) and kubelet cannot renew its node lease, so the node goes
NotReady after 40 seconds while the other apiservers are fine.

The proxy closes these connections itself. When a round's probe of an
upstream times out or fails, the proxy sends that upstream one more
`/readyz` with a `--stall-timeout` budget (3s), at most one at a time per
upstream. The upstream is stalled when this request gets no answer within
the budget and no other probe got an answer from it since the request was
sent. The proxy then looks for a witness: another upstream that has
answered 200 without a break for at least twice the stall timeout,
answered after the extra request was sent, is not warm, is present in the
EndpointSlice, has a non-negative score and is not held after a `Down` or
`Silent`. If there is one, every client connection currently proxied to
the stalled upstream is closed. Clients reconnect through the normal pick,
which prefers the witness because the stalled upstream is already in the
`failed` tier.

The check repeats every 4-5 seconds while the upstream stays silent, and
each time it closes only the connections still open to it. Nothing is
closed when:

- there is no witness, nobody is ready, or there is only one upstream;
- every upstream went silent at the same time: none of them answered
  after the extra request was sent, so none is a witness;
- the extra `/readyz` gets any answer before the budget runs out, even a
  slow one or a 500;
- the probe fails fast, for example with connection refused (an apiserver
  that exited has already reset its connections).

Connections still waiting for their first byte and connections to
fallback or `KUBERNETES_SERVICE_HOST` upstreams are never touched.

On the client side every in-flight request on the connection fails at
once and the connection is dropped. Watches resume from their last
resourceVersion without a relist, and the next request dials a new
connection that lands on a live apiserver. The close comes at most about
4.7 seconds after the hang starts: up to 1.7 seconds until a round's probe
times out, plus the stall timeout. Lease renewals then go through well
before their deadlines.

A close is logged at Warn as `closed client connections to a stalled
upstream`, with the upstream and the number of connections. Every stall
timeout is logged at Debug as `upstream stalled`, with the witness, or
`none` when nothing was closed. `/upstreams` shows `stalling` (a stall
probe is in flight) and `evictions` (how many times connections to the
upstream were closed).

This has limits. A fast 500 is not a stall: an apiserver returns it during
a graceful shutdown (and then closes its connections itself) and while it
warms up. An apiserver that hangs on requests but still answers `/readyz`
is invisible to the probes. An apiserver whose `/readyz` takes longer than
`--stall-timeout` while another one answers faster is treated as stalled,
so its clients reconnect once per check.

An upstream that disappears from the EndpointSlice is treated as `failed`
and dropped after three consecutive discovery rounds without it.

A rewritten `ca.crt` takes effect on the next probe without a restart. The
client certificate is reloaded by client-go, and a warm connection keeps
the previous certificate for up to five minutes.

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
- `GET /upstreams` → `isHealthy` (some upstream outside the `failed` tier) and the state of every upstream: `score`, `tier`, `effectiveTier`, `missing`, `lastProbe`, `warm`, `downed`, `held`, `stalling`, `evictions` (for debugging)

## CLI Flags and Environment Variables

### Core Configuration
- `--listen-addr` (env: `LISTEN_ADDRESS`, default `0.0.0.0`)
- `--listen-port` (env: `LISTEN_PORT`, default `7443`)
- `--health-listen` (env: `HEALTH_LISTEN`, default `:8080`)
- `--log-level` (env: `LOG_LEVEL`, default `info`) — `debug|info|warn|error`

### Connection Settings
- `--dial-timeout` (default `5s`): also how long a `Down` or `Silent` upstream is held in the `failed` tier; `0` dials with a 10s limit
- `--first-byte-timeout` (default `3s`): floor of the per-upstream wait for the first byte after the client's first byte on a new upstream connection, before the proxy retries another upstream; three floors cap the first-byte phase (first wait, alternative dial, second wait); `0` turns the wait off
- `--keepalive-period` (default `1s`): `0` means one minute
- `--tcp-user-timeout` (default `5s`): `0` leaves the kernel default

### Health Check Settings
- `--health-interval` (default `1s`)
- `--health-timeout` (default `500ms`), must be positive
- `--health-jitter` (default `0.2`)
- `--stall-timeout` (default `3s`): how long the extra `/readyz` to a timed-out upstream may go unanswered before its client connections are closed, provided another upstream has answered 200 for twice as long; `0` disables

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
- A probe timeout demotes an upstream but does not exclude it: under a cluster-wide slowdown traffic keeps flowing to every apiserver
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
