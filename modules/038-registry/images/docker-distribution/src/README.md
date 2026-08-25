# docker distribution, as this module builds it

This is `github.com/deckhouse/3p-distribution` at `v2.8.3-deckhouse.5`, copied in rather than cloned and
patched at build time. It is the registry the storage StatefulSet runs.

Not carried over:

- `vendor/` — the build resolves `go.mod` through GOPROXY, as this module's other images do.
- The upstream project's own image and release tooling, its documentation, and its lint configuration,
  which was in a golangci-lint format this repository's linter refuses to load. `.golangci.yaml` here
  replaces it.
- The `digest` and `registry-api-descriptor-template` commands. The image ships one binary.
- The S3 storage driver and the `silly` and `htpasswd` authenticators. Nothing configures them: the
  storage this module runs is always a filesystem directory and always authenticates with the token
  service beside it. Dropping the S3 driver also drops the AWS SDK from the module graph, which is
  most of what the vulnerability scan used to have to say about this image.

## Changes made here

The first two used to be build steps, and both are ordinary source now:

- **`SkipModeCleanup`** — a configuration flag that turns off the proxy cache's expiry scheduler
  (`configuration/configuration.go`, `registry/handlers/app.go`, `registry/proxy/proxyregistry.go`). The
  storage this module runs is not a cache with a TTL: what it holds is what the cluster needs to pull
  while the upstream is unreachable, so nothing may expire out of it. This arrived as
  `patches/001-skip-mode-cleanup.patch`, whose context had to match the fork byte for byte.
- **`scheduler.AddBlob`/`AddManifest`** are no-ops (`registry/proxy/scheduler/scheduler.go`), for the same
  reason: nothing schedules an eviction. A build script used to rewrite them with `awk` and left the old
  bodies unreachable behind an early return.
- **The write endpoint** (`configuration/configuration.go`, `registry/writeendpoint.go`,
  `registry/registry.go`): a second listener, in the same process and over the same storage, serving
  the same registry without the pull-through cache in front of it. A cache refuses every write, and
  turning it off is not an option for a store that is filled BY writes while it still has an upstream
  to serve from — so this used to be a second container with its own rendered configuration. The
  listener's configuration is derived from the serving one, which is what keeps the two halves from
  drifting apart; `registry/writeendpoint_test.go` covers what it inherits and what it must not.

## The test suite

`go test ./...` passes except for three things inherited from upstream, all of them measured here and
none of them touched by this module's changes:

- `registry/client` `TestObtainsErrorForMissingTag` expects an error message the code no longer
  produces.
- `registry/storage/driver/inmemory` runs the shared driver suite and does not finish inside Go's
  default 10-minute limit.
- `registry` `TestGracefulShutdown` binds port 5000, which on macOS belongs to AirPlay. It then leaves a
  value in the package's global signal channel, and the next test that serves anything is stopped by it
  — so the failure comes in pairs on a developer's machine and not in CI.

Two upstream test files were edited to make `go test` build at all: `Errorf`/`Fatalf` calls with a
non-constant format string, which `go vet` rejects and which therefore failed the whole package before
a single test ran.

## Updating

Diff against the upstream tag, apply what is wanted, and keep the changes above. They are commented in
place, so a three-way merge shows them as conflicts rather than losing them silently.
