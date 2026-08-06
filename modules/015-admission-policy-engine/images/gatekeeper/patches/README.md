## Patches

### 999-fix-cve.patch

Fix CVEs:
- CVE-2026-33811
- CVE-2026-27145

The patch bumps the `go` directive in the upstream `go.mod` (from `go 1.25.0` to
`go 1.25.11`) so that the Go standard library shipped in the image carries
the fixes for the CVEs above.

The `go` directive — not `toolchain` — is what sets the minimum Go version for
the build. A `toolchain` directive is silently ignored when the builder runs
with `GOTOOLCHAIN=local`, so bumping only `toolchain` does not change the
stdlib version in the resulting image and the build stays green without
actually fixing the CVEs. Bumping the `go` line instead makes any mismatch
visible: with `GOTOOLCHAIN=auto` the required toolchain is fetched and the fix
takes effect; with `GOTOOLCHAIN=local` the build fails loudly instead of
silently producing a still-vulnerable image.

The build image (`builder/golang`, see `candi/base_images.yml`) must therefore
provide Go >= 1.25.11 (or allow automatic toolchain download). When the base
builder is updated, bump the `go` line here to match the patched stdlib
version that actually closes the CVEs, and verify the stdlib version in the
SBOM of the built image matches what this patch implies.
