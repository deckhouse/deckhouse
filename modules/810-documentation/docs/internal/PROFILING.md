# Profiling docs-builder

docs-builder renders module versions on demand. Its CPU profile is in `docs/site/backends/docs-builder/default.pgo`, and `go build` applies it (`-pgo=auto`).

## Which images have pprof

- Images built without a release tag (`CI_COMMIT_TAG` is empty): merge requests (`pr<MR>`), branches, `main`. docs-builder is built with the `pprof` Go build tag and serves pprof on `127.0.0.1:6060`.
- Release images (`CI_COMMIT_TAG` is `vX.Y.Z`) and the image of the public site: no `pprof` tag. The binary does not contain `net/http/pprof` and listens on no profiling port.

The switch is the `go build` line in `docs/site/werf-docs-builder.inc.yaml`; the code is `docs/site/backends/docs-builder/internal/pprof`.
The Deployment has no port for pprof in either case: the server listens on the loopback of the pod, other pods can not reach it.

## Take a CPU profile

1. Put the image of the merge request on a test cluster: `d8settag pr<MR>`.
2. Forward the port of one builder pod:

   ```bash
   d8 k -n d8-system get pods -l app=documentation
   d8 k -n d8-system port-forward pod/<pod> 6060
   ```

3. Check that pprof answers: `curl -s 'http://127.0.0.1:6060/debug/pprof/' | head`.
4. Take a profile while the load you want to profile runs, for example readers opening versions or a first pass after a restart:

   ```bash
   go tool pprof -proto 'http://127.0.0.1:6060/debug/pprof/profile?seconds=120' > cpu-1.pprof
   ```

   Quote the URL: in zsh `?` is a glob character.

## Refresh default.pgo

1. Take several profiles of a realistic load: with S3 and without, the first pass of the pinned versions and readers opening versions.
2. Merge them and put the result next to `main.go`, from `docs/site/backends/docs-builder`:

   ```bash
   go tool pprof -proto cpu-*.pprof > default.pgo
   ```

3. Check, from the same directory, that the build applies it: `go build -o /tmp/docs-builder . && go version -m /tmp/docs-builder | grep pgo`.
