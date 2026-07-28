## Patches

### 001-go-mod.patch

Update dependencies to fix CVEs
- [CVE-2025-47914](https://github.com/advisories/GHSA-f6x5-jh6r-wrfv)
- [CVE-2025-58181](https://github.com/advisories/GHSA-j5w8-q4qc-rx2x)
- [CVE-2025-47908](https://github.com/advisories/GHSA-mh55-gqvf-xfwm)

Also bump `golang.org/x/crypto`→v0.53.0, `golang.org/x/net`→v0.56.0, `golang.org/x/sys`→v0.46.0,
`golang.org/x/text`→v0.39.0 and `go.mongodb.org/mongo-driver`→v1.17.7 to fix CVEs reported by Trivy
against the embedded Go dependencies:
- x/crypto: CVE-2026-39828/39829/39830/39831/39832/39835, CVE-2026-42508, CVE-2026-46595/46597
- x/net: CVE-2026-25680/25681/27136/33814/39821, CVE-2026-42502/42506, CVE-2026-46600
- x/sys: CVE-2026-39824
- x/text: CVE-2026-56852
- mongo-driver: CVE-2026-2303
