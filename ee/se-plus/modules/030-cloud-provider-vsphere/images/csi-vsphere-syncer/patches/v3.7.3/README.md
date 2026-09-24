## Patches

## 001-go-mod.patch

Bump go.mod dependencies to fix known CVEs:
- GO-2026-6061, GO-2026-6348, GO-2026-6441, GO-2026-6443: google.golang.org/grpc -> v1.83.2
- GO-2026-5942: golang.org/x/net -> v0.58.0 (required by grpc)
- GO-2026-5970: golang.org/x/text -> v0.41.0 (required by grpc)
