# Patches

## 001-go-mod.patch

Dependency bumps that close CVEs in the vendored module graph of Loki v2.9.15.
Only `go.mod` and `go.sum` are touched.

| CVE | module | v2.9.15 pins | bumped to |
| --- | --- | --- | --- |
| CVE-2026-33186, GHSA-hrxh-6v49-42gf, CVE-2026-84303, CVE-2026-84304, CVE-2026-84445 | `google.golang.org/grpc` | v1.59.0 | v1.83.2 |
| CVE-2026-32285 | `github.com/buger/jsonparser` | v1.1.1 | v1.1.2 |
| CVE-2026-2303 | `go.mongodb.org/mongo-driver` (indirect) | v1.11.2 | v1.17.7 |
| GHSA-w67g-5rqw-f597 | `github.com/gorilla/websocket` | v1.5.0 | v1.5.3 |
| CVE-2026-39827 … CVE-2026-39835, CVE-2026-42508, CVE-2026-46595, CVE-2026-46597, CVE-2026-46598, CVE-2026-56854 | `golang.org/x/crypto` | v0.36.0 | v0.55.0 |
| CVE-2026-25680, CVE-2026-25681, CVE-2026-27136, CVE-2026-33814, CVE-2026-39821, CVE-2026-42502, CVE-2026-42506, CVE-2026-46600 | `golang.org/x/net` | v0.38.0 | v0.58.0 |
| CVE-2026-39824 | `golang.org/x/sys` | v0.31.0 | v0.47.0 |
| CVE-2026-56852 | `golang.org/x/text` | v0.23.0 | v0.41.0 |

`golang.org/x/crypto` is deliberately capped at v0.55.0: v0.56.0 declares `go 1.26.0`,
while the image builds with `builder/golang-alpine`, which is Go 1.25 in
`candi/base_images.yml` on this branch. The two findings that are only fixed in v0.56.0
(CVE-2026-56855, CVE-2026-78662) live in `golang.org/x/crypto/ssh`, which is not linked
into the binary, and are waived in `known_vulnerabilities.vex` instead.

Requires 004-grpc-health-list.patch to compile.

### Regenerating the patch

The patch must be a diff against a **pristine** v2.9.15 tree and must apply with plain
`git apply` (the build in `images/loki/werf.inc.yaml` passes no `--3way`). So apply the
patches into the worktree and never commit them:

```sh
git clone --depth 1 --branch v2.9.15 https://github.com/grafana/loki.git /tmp/loki-src
cd /tmp/loki-src
git apply --verbose /path/to/this/repo/modules/462-loki/images/loki/patches/*.patch
rm -rf vendor
export GOFLAGS=-mod=mod          # go get refuses to edit go.mod under the implicit -mod=vendor
export GOTOOLCHAIN=local         # keep the go directive from drifting to the local toolchain
# ... run `go get` for the module you are bumping, then: ...
go mod vendor                    # must succeed; resolve any "missing go.sum entry" it reports
git --no-pager diff --no-color --no-ext-diff -- go.mod go.sum \
  > /path/to/this/repo/modules/462-loki/images/loki/patches/001-go-mod.patch
```

Then assert: exactly two `diff --git` headers (go.mod, go.sum); the `go 1.23.0` +
`toolchain go1.23.2` pair is still replaced by a bare `go 1.25.0` with no `+toolchain`
line; and `go version -m` on the built binary reports the versions you intended.
Do **not** run `go mod tidy` — on Loki 2.9 with a modern toolchain it produces large
unrelated churn and rewrites the `go`/`toolchain` directives.

Prefer bumping **on top of the existing patch** (apply it, `go get`, re-diff), not
regenerating the whole thing from pristine: the recipe below is a record of how each bump
was derived, not a reproducible script — `go mod vendor` pulls extra `go.sum` entries whose
exact set depends on the toolchain, so a from-scratch run drifts.

### How each bump was derived

```sh
# CVE-2026-33186: authorization bypass via missing leading slash in :path (fixed in 1.79.3).
go get golang.org/x/crypto@v0.54.0
go get golang.org/x/net@v0.57.0
go get google.golang.org/grpc@v1.79.3
# google.golang.org/api v0.149.0 does not compile against the new
# credentials/google.DefaultCredentialsOptions, so it has to go up as well;
# it pulls grpc to v1.83.0, which is also >= v1.79.3.
go get google.golang.org/api@v0.293.0
# go.sum entries missing for packages that `go mod vendor` walks into
go get github.com/envoyproxy/go-control-plane/envoy/service/status/v3@v1.37.0
go get cloud.google.com/go/pubsub/apiv1/pubsubpb@v1.50.1

# CVE-2026-32285: negative slice index -> panic in jsonparser.Delete.
# v1.1.2 has no requires and an API identical to v1.1.1, so the module graph does not move.
# Deliberately not v1.6.x: parser.go is rewritten there and this is Loki's hottest path.
go get github.com/buger/jsonparser@v1.1.2

# CVE-2026-2303: heap out-of-bounds read in the CGo GSSAPI wrapper. Indirect dependency,
# reached via prometheus/notifier -> go-openapi/strfmt -> mongo-driver/bson; only the bson
# subtree is vendored. v1.11.2 is also retracted upstream.
go get go.mongodb.org/mongo-driver@v1.17.7

# CVE-2026-46595 / CVE-2026-56854: source-address restrictions returned by SSH auth
# callbacks were not enforced. Fixed in v0.55.0, which is the last release that still
# declares `go 1.25.0`. Pulls x/mod v0.38.0, x/text v0.41.0 and x/tools v0.48.0.
go get golang.org/x/crypto@v0.55.0

# CVE-2026-84303, CVE-2026-84304 (heap exhaustion via HTTP/2 DATA frame fragmentation)
# and CVE-2026-84445 (index-out-of-bounds panic in xDS RouteAndProcess on a request with
# neither :authority nor Host). Fixed in v1.83.1 and v1.83.2 respectively.
# v1.83.2 requires golang.org/x/net v0.58.0, which it pulls in itself.
go get google.golang.org/grpc@v1.83.2

# GHSA-w67g-5rqw-f597: integer overflow in the permessage-deflate handling of
# github.com/gorilla/websocket. v1.5.3 has no requires, so the module graph does not move.
go get github.com/gorilla/websocket@v1.5.3

go mod vendor
```

## 002-Allow-delete-logs.patch

Enable/disable `/loki/api/v1/delete` endpoints by setting `ALLOW_DELETE_LOGS` env value to true/false.

## 003-Force-expiration.patch

Automatically delete old logs by setting `force_expiration_threshold` higher than 0.

## 004-grpc-health-list.patch

Required by the grpc bump in 001-go-mod.patch: `grpc_health_v1.HealthServer` gained the `List`
method, which the pinned `github.com/grafana/dskit` `grpcutil.HealthCheck` does not implement.
Wraps it into a shim that reports the overall serving status.
