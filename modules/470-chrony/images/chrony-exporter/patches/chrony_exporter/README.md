## patches

### 001-go-mod.patch

Bumps golang.org/x/net and golang.org/x/text (carrying x/crypto and x/sys) to
clear the CVEs reported against the chrony_exporter binary. Keeps
github.com/prometheus/exporter-toolkit at v0.13.0 on purpose: from v0.14.0 its
landing page answers 404 on any path but the route prefix, and the DaemonSet's
liveness/readiness probes hit /healthz, which this exporter never serves. That
is why chrony_exporter stays at v0.11.0 instead of being bumped (v0.14.0 was
tried in #22115 and reverted in #22170).

### 002-go-mod.patch

Bumps golang.org/x/crypto to v0.55.0 and golang.org/x/net to v0.58.0 (carrying
x/sync v0.22.0, x/sys v0.47.0, x/text v0.41.0) to clear the CVEs reported
against the chrony_exporter binary in the cse-1.73 scan. Raises the go
directive to 1.25.0 and drops the toolchain line, which is what those versions
require; that is the ceiling of builder/golang-alpine in this branch.
