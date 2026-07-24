## Patches

### 000-go-mod.patch

Remove the `godebug tlskyber=0` directive from go.mod. This setting was introduced
in Go 1.23 to disable the experimental X25519Kyber768Draft00 key exchange but was
removed in Go 1.24 when tlskyber became tlsmlkem. The directive is invalid in Go 1.24+
and must be removed before applying the go-mod bump patch.

Bump go.mod dependencies to fix known CVEs:
- golang.org/x/crypto v0.54.0 (CVE-2026-39827, CVE-2026-39828, CVE-2026-39829, CVE-2026-39830, CVE-2026-39831, CVE-2026-39832, CVE-2026-39833, CVE-2026-39834, CVE-2026-39835, CVE-2026-42508, CVE-2026-46595, CVE-2026-46597, CVE-2026-46598; GO-2026-5932 has no fix and is documented via VEX)
- golang.org/x/net v0.57.0 (CVE-2026-25680, CVE-2026-25681, CVE-2026-27136, CVE-2026-33814, CVE-2026-39821, CVE-2026-42502, CVE-2026-42506, CVE-2026-46600)
- golang.org/x/sys v0.47.0 (CVE-2026-39824)
- github.com/cloudflare/circl v1.6.3 (CVE-2025-8556, CVE-2026-1229)
- google.golang.org/grpc v1.79.3 (CVE-2026-33186)
- Also bumps AWS SDK v2, smithy-go, google/go-cmp, opentelemetry, protobuf, and other dependencies to their latest compatible versions.
