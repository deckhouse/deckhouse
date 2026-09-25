## Patches

### 000-gomod.patch

dependency bump in `server`, `etcdctl`, `etcdutl` go.mod/go.sum: `golang.org/x/net` → v0.58.0, `golang.org/x/sys` → v0.47.0, `golang.org/x/text` → v0.41.0, `golang.org/x/crypto` → v0.55.0 (`etcdutl`/`server` only).

Applied first, before `001-etcdctl-snapshot-pipe.patch`, per the repo-wide convention of numbering dependency patches `000-*`.

### 001-etcdctl-snapshot-pipe.patch

feature: support for piping snapshot to stdout
