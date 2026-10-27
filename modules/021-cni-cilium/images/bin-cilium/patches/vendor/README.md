# Patches for vendored dependencies

`go mod vendor` regenerates `vendor/` after the source patches in the parent directory are
applied, so these patches are applied afterwards, right after `go mod vendor`, in every image
built from these sources: `bin-cilium`, `operator` and `cilium-hubble/relay`.

Each patch is tied to the module version in `vendor/modules.txt`. When `000-go-mod.patch`
bumps that module, regenerate the patch or drop it if upstream has the fix.

## 001-statedb-close-remote-response-body.patch

`github.com/cilium/statedb` v0.4.5, HANDLE_LEAK: `RemoteTable.query()`/`Changes()` never
closed the HTTP response body, so the connection was held after a decode error or an early
stop of the iteration. The body is now closed by the iterator, as upstream does since
statedb v0.9.0. Used by `cilium-dbg`.

## 002-renameio-cleanup-tempfile-on-error.patch

`github.com/google/renameio/v2` v2.0.0, HANDLE_LEAK.EX: `NewPendingFile()` returned on a
`Stat`/`Chmod` error without closing and removing the temporary file. It now calls
`PendingFile.Cleanup()`. Not fixed upstream (still present in v2.0.2).
