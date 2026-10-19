## Patches

No patches are applied to terraform-provider-aws v6.69.1.

The go.mod bumps carried for v5.83.1 (`000-go-mod.patch`, `001-go-mod.patch`) are dropped:
upstream v6.69.1 already requires the same or newer versions of every module they pinned for
CVE fixes (golang.org/x/crypto v0.58.0, golang.org/x/net v0.61.0, golang.org/x/sys v0.49.0,
golang.org/x/text v0.43.0, golang.org/x/mod v0.42.0, google.golang.org/grpc v1.85.0-dev,
github.com/cloudflare/circl v1.6.5). The `godebug tlskyber=0` directive removed by
`000-go-mod.patch` is no longer in upstream go.mod.

Put new fixes here as `NNN-*.patch`; `werf.inc.yaml` applies every `/patches/*.patch` in order
when the directory has any.
