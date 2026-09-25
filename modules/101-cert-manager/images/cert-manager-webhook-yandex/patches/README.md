## Patches

Applied on top of the 3p mirror
`yandex-cloud/cert-manager-webhook-yandex` at the commit pinned in `modules/101-cert-manager/oss.yaml`
(`1a7dd500a97e9206fa5cfebb102decd3889e3544`).

### 001-ha-and-zone-resolution.patch

Deckhouse hardening of the upstream ACME DNS-01 webhook:

- rebuild Yandex SDK client in `CleanUp` (avoid nil client on another replica)
- shut down `ycsdk.SDK` after each challenge (fresh bounded context)
- resolve zone via `ChallengeRequest.ResolvedZone` + pagination + exact public-zone match
- reject unsafe characters in zone names used in the Yandex filter expression
- unit tests for `normalizeZone` / `getDNSZone`
- keep `go.mod` / `go.sum` in sync so the image build can use `go mod download` without `go mod tidy`

### 999-FixCVE.patch

Fix CVEs:
- CVE-2026-56854
- CVE-2026-56855
- CVE-2026-78662
- CVE-2026-81870
- CVE-2026-84303
- CVE-2026-84304
- CVE-2026-84445

GHSA:
- GHSA-hrxh-6v49-42gf
