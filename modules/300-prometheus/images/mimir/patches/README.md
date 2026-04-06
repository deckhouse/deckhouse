## Patches

### 001-go-mod.patch

Update dependencies to fix CVEs
- [CVE-2025-47914](https://github.com/advisories/GHSA-f6x5-jh6r-wrfv)
- [CVE-2025-58181](https://github.com/advisories/GHSA-j5w8-q4qc-rx2x)
- [CVE-2025-22868](https://github.com/advisories/GHSA-6v2p-p543-phr9)
- [CVE-2025-27144](https://github.com/advisories/GHSA-c6gw-w398-hv78)
- [CVE-2025-30204](https://github.com/advisories/GHSA-mh63-6h87-95cp)
- [CVE-2024-45339](https://github.com/advisories/GHSA-6wxm-mpqj-6jpf)

### 002-fix-cves.patch

Fix CVE-2026-33186 and CVE-2026-34986.
```sh
go get google.golang.org/grpc@v1.79.3
go get github.com/go-jose/go-jose/v4@v4.1.4
go mod tidy
```
