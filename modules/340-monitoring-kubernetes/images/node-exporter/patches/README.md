## Patches

### 001-go-mod.patch

Update dependencies to fix CVEs
- [CVE-2025-47914](https://github.com/advisories/GHSA-f6x5-jh6r-wrfv)
- [CVE-2025-58181](https://github.com/advisories/GHSA-j5w8-q4qc-rx2x)
- [CVE-2025-52881](https://github.com/advisories/GHSA-cgrx-mc8f-2prm)

Bump `golang.org/x/crypto` to v0.53.0, `golang.org/x/net` to v0.56.0, `golang.org/x/sys` to v0.46.0
and `golang.org/x/text` to v0.39.0 (and their transitive `golang.org/x/sync`) to fix CVEs reported
by Trivy against the embedded Go dependencies:
- `golang.org/x/crypto`: CVE-2026-39828/39829/39830/39831/39832/39835, CVE-2026-42508, CVE-2026-46595/46597
- `golang.org/x/net`: CVE-2026-25680/25681/27136/33814/39821, CVE-2026-42502/42506, CVE-2026-46600
- `golang.org/x/sys`: CVE-2026-39824
- `golang.org/x/text`: CVE-2026-56852
