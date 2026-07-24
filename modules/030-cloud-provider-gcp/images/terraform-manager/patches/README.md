## Patches

### 000-remove-scripts-diff.patch

Removes scripts/diff.go from the upstream source tree. This file imports a
private package (github.com/hashicorp/terraform-provider-clean-google) that
cannot be resolved by go mod tidy. The file is also removed at build time by
werf.inc.yaml; this patch mirrors that removal so that go mod tidy can
complete during patch generation.

### 001-go-mod.patch

Bump go.mod dependencies to fix known CVEs:
- CVE-2024-45339: github.com/golang/glog v1.1.2 → v1.2.5
- CVE-2025-65637: github.com/sirupsen/logrus v1.8.1 → v1.9.3
- CVE-2025-22868: golang.org/x/oauth2 v0.17.0 → v0.34.0
- CVE-2026-33186: google.golang.org/grpc v1.61.1 → v1.79.3
- CVE-2024-24786: google.golang.org/protobuf v1.32.0 → v1.36.10
- golang.org/x/net → v0.55.0 (via replace directive)
- golang.org/x/crypto → v0.52.0 (via replace directive; GO-2026-5932 has no fix, see known_vulnerabilities.vex)

### 002-remove_routes_on_deletion.patch

https://github.com/flant/terraform-provider-google/compare/v3.48.0...v3.48.0-flant.1
