### 001-go-mod.patch

Update dependencies to fix CVE-2025-47914, CVE-2025-58181

Also bump `golang.org/x/crypto`→v0.53.0, `golang.org/x/net`→v0.56.0, `golang.org/x/sys`→v0.46.0 and
`golang.org/x/text`→v0.39.0 to fix CVEs reported by Trivy against the embedded Go dependencies:
- x/crypto: CVE-2026-39828/39829/39830/39831/39832/39835, CVE-2026-42508, CVE-2026-46595/46597
- x/net: CVE-2026-25680/25681/27136/33814/39821, CVE-2026-42502/42506, CVE-2026-46600
- x/sys: CVE-2026-39824
- x/text: CVE-2026-56852
