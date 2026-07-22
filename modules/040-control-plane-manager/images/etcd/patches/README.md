## Patches

### 000-gomod.patch

dependency bump: `golang.org/x/net` v0.54.0 → v0.55.0 in `server`, `etcdctl`, `etcdutl` go.mod/go.sum (fixes CVE-2026-25681, CVE-2026-27136, CVE-2026-39821, CVE-2026-42502, CVE-2026-25680, CVE-2026-42506, CVE-2026-33814). `golang.org/x/crypto` and `golang.org/x/sys` were already at fixed versions (v0.52.0 / v0.45.0) and required no change.

Applied first, before `001-etcdctl-snapshot-pipe.patch`, per the repo-wide convention of numbering dependency patches `000-*`.

### 001-etcdctl-snapshot-pipe.patch

feature: support for piping snapshot to stdout
