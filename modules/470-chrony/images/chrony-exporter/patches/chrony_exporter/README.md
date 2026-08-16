## patches

### 001-go-mod.patch

Bumps golang.org/x/net and golang.org/x/text (carrying x/crypto and x/sys) to
clear the CVEs reported against the chrony_exporter binary, including
CVE-2025-58181. Keeps github.com/prometheus/exporter-toolkit at v0.13.0 on
purpose: from v0.14.0 its landing page answers 404 on any path but the route
prefix, and the DaemonSet's liveness/readiness probes hit /healthz, which this
exporter never serves.
