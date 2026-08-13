# Patches

## 001-go-mod.patch

Update dependencies

## 002-already_logged.patch

patch

### 003-fix-cves.patch

Fixes CVE-2025-22868 CVE-2024-28180 CVE-2025-47914 CVE-2025-58181

Update Go module dependencies to fix:
- CVE-2026-39828 (`golang.org/x/crypto` -> `v0.53.0`)
- CVE-2026-39824 (`golang.org/x/sys` -> `v0.46.0`)
- CVE-2026-56852 (`golang.org/x/text` -> `v0.39.0`)

(previously split across 003/004/005; consolidated into one patch)
