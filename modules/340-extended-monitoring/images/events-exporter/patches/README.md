### 001-go-mod-logrus.patch

Force `github.com/sirupsen/logrus` to v1.9.3 via a `replace` directive in `go.mod` to fix CVE-2025-65637,
and drop the stale `logrus v1.2.0`, `v1.4.2` and `v1.6.0` `/go.mod` checksums from `go.sum`.
