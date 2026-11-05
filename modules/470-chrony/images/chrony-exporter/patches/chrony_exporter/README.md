## patches

### 001-go-mod.patch

Bumps golang.org/x/crypto from v0.55.0 to v0.58.0 for CVE-2026-56855 and
CVE-2026-78662. x/crypto v0.56.0 and later require go 1.26.0, so the go
directive moves from 1.25.0 to 1.26.0 and the image needs a Go 1.26+ builder.

Bumps golang.org/x/net from v0.57.0 to v0.62.0 for CVE-2026-78659,
CVE-2026-78660, CVE-2026-78661, CVE-2026-78663, CVE-2026-78669,
CVE-2026-97032 and CVE-2026-97045. This also moves golang.org/x/sync,
golang.org/x/sys and golang.org/x/text to the versions x/net v0.62.0 requires.
