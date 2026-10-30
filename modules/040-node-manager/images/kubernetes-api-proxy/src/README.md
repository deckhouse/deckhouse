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

`upstream.List` probes every upstream once per round with `GET /readyz`
and a `--health-timeout` budget (500ms). Probes share one keep-alive HTTPS
transport from client-go (HTTP/2 when the apiserver offers it),
so a probe normally travels over a warm connection.

Each probe connection has `TCP_USER_TIMEOUT` equal to `--health-timeout`
(Linux only). When the network drops packets without a reset, the kernel
closes the connection after this time, and the next probe dials a new one.
Without it, the probes wait on the old connection for the next kernel
retransmission, and an upstream becomes ready again up to a few seconds
after the network is back.

The rounds run one after another. A round starts `--health-interval`
±`--health-jitter` (1s ±20%) after the start of the previous round. If the
previous round ends later, the round starts when the previous round ends.
With the defaults a round takes not more than `--health-timeout` (500ms), so
in normal mode the next round always starts on the interval.
The first round starts at once, the second after a random part of the
interval.

A round sends one probe to each upstream. An upstream that does not answer
`ready` fails the round.

Fail-step. A round in which no upstream answered `ready` starts a failure
episode. After a round that ends in the first `--fail-step-window` (30s) of
the episode, the next round starts `--fail-step-interval` (500ms) with the
same jitter after the start of that round, or when that round ends if it ends
later. A round with a hung upstream takes `--health-timeout`, so with the
defaults the rounds start every 500 to 600ms. When the window ends, the
normal interval applies until the episode ends. The first round with a `ready` upstream ends the episode; the next failure starts a
new episode with a new window. `--fail-step-window=0` disables fail-step.

The proxy logs the start and the end of fail-step at Warn, once per episode.
`fail-step started` has `reason` (`no upstream is ready`), `interval` and
`window`. `fail-step stopped` has `reason` (`an upstream is ready` or
`the fail-step window ended`) and `duration` (the time from the start of the
episode). The first round after the end of the window logs the end: with
`the fail-step window ended` if no upstream is ready, or with
`an upstream is ready` if one is. So `duration` can be a little longer than
the window.

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

The check repeats every 3 to 4.2 seconds while the upstream stays silent:
the stall timeout plus up to 1.2 seconds until the next round applies. Each
time it closes only the connections still open to it. Nothing is
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
4.7 seconds after the hang starts: up to 1.2 seconds until the next round
starts, up to 0.5 seconds until its probe times out, plus the stall
timeout. Lease renewals then go through well
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
  --listen-address=0.0.0.0 \
  --listen-port=7443 \
  --health-listen=127.0.0.1:6480 \
  --log-level=info
```

## Health Endpoints

- `GET /healthz` → always `200 OK`
- `GET /readyz` → always `200 OK`: with no ready upstream the proxy still sends connections to the upstream with the best state (also from the `failed` tier); only with an empty upstream list it sends them to the fallback upstreams or to the default Kubernetes Service
- `GET /upstreams` → a JSON object:
  - `isHealthy`: some upstream is outside the `failed` tier.
  - `probeSeq`: `CLOCK_BOOTTIME` in ms at the start of the last applied probe round; strictly increases. It is one number for all upstreams. If the clock gives the same value as for the previous started round, or less, the number is the previous number plus 1. So a round with a number larger than a moment T on the same clock started after T. The number grows also after a proxy restart, and a change of the wall clock has no effect on it. `0` means that no round is complete.
  - `probeOK`: the result of that round. `true` when at least one upstream answered `ready`; `false` when no upstream did or the upstream list is empty.
  - `nodes`: the state of every upstream: `score`, `tier`, `effectiveTier`, `missing`, `lastProbe`, `warm`, `downed`, `held`, `stalling`, `evictions` (for debugging) and `lastReadyAgoMs`.
    - `lastReadyAgoMs`: the time in milliseconds since the last probe of this upstream that answered `ready`. The time starts at the answer, not at the end of the round: a slow upstream in the same round does not delay it. The field is not there when the upstream did not answer `ready` since the proxy started.

  `probeSeq`, `probeOK` and `lastReadyAgoMs` are critical for fencing. The fencing agent of node-manager reads them on every node to decide if the node sees the control plane: `probeSeq` tells that a round started after a given moment, `probeOK` gives the result of that round, and `lastReadyAgoMs` gives the age of the last `ready` answer. Do not remove these fields or change their meaning without a change in the fencing agent.

  `probeSeq`, `probeOK` and `lastReadyAgoMs` came in one version of the proxy. A response with `probeSeq` comes from a proxy that has these three fields.

  Example (the `nodes` item is shortened):

  ```json
  {
    "isHealthy": true,
    "probeSeq": 8123456,
    "probeOK": true,
    "nodes": [
      {
        "upstream": "10.0.0.1:6443",
        "score": 3,
        "tier": 1,
        "lastProbe": "ready",
        "lastReadyAgoMs": 310
      }
    ]
  }
  ```

## CLI Flags and Environment Variables

### Core Configuration
- `--listen-address` (env: `LISTEN_ADDRESS`, default `127.0.0.1`)
- `--listen-port` (env: `LISTEN_PORT`, default `6445`)
- `--health-listen` (env: `HEALTH_LISTEN`, default `127.0.0.1:6480`): the address of `/healthz`, `/readyz` and `/upstreams`. The static pod manifest of bashible (step `051_pull_and_configure_kubernetes_api_proxy`) sets the same address, and the dhctl preflight check of the ports (`candi/bashible/preflight/check_ports.sh.tpl`) checks that the port is free before the bootstrap
- `--log-level` (env: `LOG_LEVEL`, default `info`) — `debug|info|warn|error`
- `--shutdown-delay` (default `3s`): how long the proxy continues to serve after the first `SIGTERM` or `SIGINT` before it closes the listener; see the Shutdown section below. `0` closes the listener at once

### Connection Settings
- `--dial-timeout` (default `5s`): also how long a `Down` or `Silent` upstream is held in the `failed` tier; `0` dials with a 10s limit
- `--first-byte-timeout` (default `3s`): floor of the per-upstream wait for the first byte after the client's first byte on a new upstream connection, before the proxy retries another upstream; three floors cap the first-byte phase (first wait, alternative dial, second wait); `0` turns the wait off
- `--keepalive-period` (default `1s`): `0` means one minute
- `--tcp-user-timeout` (default `5s`): `0` leaves the kernel default

### Health Check Settings

The static pod manifest does not set these flags, thus the defaults apply on every node.

- `--health-interval` (default `1s`), must be positive: the time between the starts of two probe rounds, or more if a round takes longer
- `--health-timeout` (default `500ms`), must be positive
- `--health-jitter` (default `0.2`), in `[0, 1]`: applies to the normal and the fail-step interval
- `--fail-step-interval` (default `500ms`), must be positive: the time between the starts of two rounds during fail-step, or more if a round takes longer
- `--fail-step-window` (default `30s`), must not be negative: the longest fail-step period in one failure episode; `0` disables fail-step
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
            - "--listen-address=0.0.0.0"
            - "--listen-port=6445"
            - "--health-listen=127.0.0.1:6480"
            - "--log-level=debug"
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
              host: 127.0.0.1
            initialDelaySeconds: 2
            periodSeconds: 5
          livenessProbe:
            httpGet:
              path: /healthz
              port: health
              host: 127.0.0.1
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
        - "--health-listen=127.0.0.1:6480"
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
          host: 127.0.0.1
        initialDelaySeconds: 2
        periodSeconds: 5
      livenessProbe:
        httpGet:
          path: /healthz
          port: health
          host: 127.0.0.1
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

- `/healthz` and `/readyz` always answer `200`; the upstream state is only in `/upstreams`
- A probe timeout demotes an upstream but does not exclude it: under a cluster-wide slowdown traffic keeps flowing to every apiserver
- Jitter (`--health-jitter`) staggers health checks to reduce synchronized load
- TLS settings apply to health checks only; data path is raw TCP proxying

### Shutdown

On a node shutdown kubelet sends `SIGTERM` to the proxy and to the other node-critical pods at the same time. These pods and kubelet write their last state to the apiserver through the proxy. Thus after the first `SIGTERM` or `SIGINT` the proxy continues to work for `--shutdown-delay` (3s) and logs `stop signal received, serve until the shutdown delay ends`:

- the listener accepts and proxies new connections, and open connections continue to work;
- the probes, fail-step and discovery continue;
- `/healthz`, `/readyz` and `/upstreams` continue to answer `200`.

Then the proxy logs `shutting down...` and stops discovery. Then it closes the health server, which gets at most 1s to close its connections. Then it closes the listener and stops the probes and fail-step. Open proxied connections are not closed by the proxy: they end when the process exits. A second `SIGTERM` or `SIGINT` ends the delay at once (log `second stop signal received, end the shutdown delay`).

The static pod has the priority `2000001000` (`system-node-critical`) and no `terminationGracePeriodSeconds`, thus its grace period is 30s. On a node shutdown kubelet uses the smaller period of the priority group from `shutdownGracePeriodByPodPriority` (bashible step `064_configure_kubelet`). When this period ends, kubelet kills the process (`SIGKILL`). The time from the first signal until `127.0.0.1:6445` closes is:

| Event | Grace period | Port `6445` closes after the signal |
|---|---|---|
| Worker shutdown | 5s | at 3s to 4s: the proxy closes it |
| Master shutdown | 25s | at 3s to 4s: the proxy closes it |
| Pod deletion or manifest change | 30s | at 3s to 4s: the proxy closes it |

The delay (3s) and the shutdown of the health server (at most 1s) take less than the smallest group period (5s on a worker). Thus on workers and on masters the proxy closes the listener itself and the log has the `shutting down...` line.

On a pod deletion or a manifest change kubelet starts the new container after the old one stops. Thus the delay makes the restart 3s longer, but the time while the port is closed does not change.

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
