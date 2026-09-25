### 001-ignore-static-nodes.patch

This patch is for our case when we want to have a static Nodes in the cluster, managed by vSphere cloud provider.

> Consider implementing a flag in CCM config and sending as a PR to the upstream.

Update klog to klog/v2 in pkg/ccm/instances.go

### 002-add-vapptemplate-search-by-vapptemplate-id.patch

TODO

Update klog to klog/v2 in pkg/vcdsdk/vapp.go

### 003-go-mod.patch

Bump go.mod dependencies to fix known CVEs.

### 004-klog.patch

Update klog to klog/v2 in other files

### 005-add-vapptemplate-search-by-org.patch

Add support for searching vAppTemplates in a given org

### 006-fix-lb-health-monitor.patch

Fixes TCP health monitors removal during an update of the pool

### 007-support-load-balancer-ip-annotation.patch

Files:

- pkg/ccm/loadbalancer.go

Changes:

- Add support for the `vcd.cpi.flant.com/load-balancer-ip` annotation

### 008-fix-ccm-command-signature.patch

Fix NewCloudControllerManagerCommand call signature for k8s.io v0.34.3

In k8s.io v0.34.3, the function signature changed to include an additional `map[string]string` parameter for feature gates between `DefaultInitFuncConstructors` and `NamedFlagSets`.


## 009-go-mod.patch

Bump dependencies to close CVE: `github.com/google/cel-go` to v0.29.0 (GHSA-gcjh-h69q-9w9g),
`golang.org/x/crypto` to v0.56.0 (CVE-2026-56854, CVE-2026-56855, CVE-2026-78662) and
`google.golang.org/grpc` to v1.83.1 (CVE-2026-84304, GHSA-hrxh-6v49-42gf).
Layered on top of `003-go-mod.patch`, which already lifts the OpenTelemetry family to v1.43.0;
regenerating the whole graph from upstream fails because the old `k8s.io/apiserver` in 1.6.1
pulls the removed monolithic `go.opentelemetry.io/otel/exporters/otlp` packages.
Contains `go.mod` + `go.sum` only.
