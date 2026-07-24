## 001-implement_openstack_compute_servergroup_v2_data_source.patch
add data source for openstack_compute_servergroup_v2

## 002-go-mod.patch

Bump go.mod dependencies to fix known CVEs:
- golang.org/x/net v0.55.0 (CVE-2026-25680, CVE-2026-25681, CVE-2026-27136, CVE-2026-33814, CVE-2026-39821, CVE-2026-42502, CVE-2026-42506)
- golang.org/x/sys v0.45.0 (CVE-2026-39824)
- golang.org/x/crypto v0.52.0 (indirect, pulled by x/net upgrade)
- github.com/cloudflare/circl v1.6.3 (CVE-2025-8556, CVE-2026-1229)
- github.com/ulikunitz/xz v0.5.15 (CVE-2025-58058)
- google.golang.org/grpc v1.79.3 (CVE-2026-33186)
- google.golang.org/protobuf v1.36.10 (CVE-2024-24786)
- github.com/stretchr/testify v1.8.1, github.com/google/go-cmp v0.7.0, github.com/golang/protobuf v1.5.4 (maintenance bumps)

MODULE_REPLACES applied: golang.org/x/net v0.55.0 and golang.org/x/crypto v0.52.0 (force pinned as indirect deps were downgraded by go mod tidy).

## 003-empty-metadata-fix.patch
Empty metadata always create diff. Set empty map instead nil for metadata when read resource.
