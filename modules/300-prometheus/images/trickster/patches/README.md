## Patches

## 001-go-mod.patch

Update dependencies.

Bump `golang.org/x/net`→v0.56.0 and `golang.org/x/sys`→v0.46.0 (and transitive `golang.org/x/text`
→v0.39.0, `golang.org/x/crypto`→v0.53.0) to fix CVEs reported by Trivy against the embedded Go
dependencies:
- x/net: CVE-2025-47911, CVE-2025-58190, CVE-2026-25680/25681/27136/33814/39821, CVE-2026-42502/42506, CVE-2026-46600
- x/sys: CVE-2026-39824
