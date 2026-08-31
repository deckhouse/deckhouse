## Istio image build targets

The module builds version-specific images for the supported Istio releases:

- `common`, `cni`, `istioctl`, `kiali`, `pilot`, and `proxyv2` for 1.25.2, 1.27.9, and 1.29.6;
- `operator` for the operator-backed Istio 1.25.2 release.

Each target is defined in its corresponding `*-v1x<minor>x<patch>/werf.inc.yaml` directory. Images are built from the upstream Istio sources pinned by that target, with Deckhouse patches and vulnerability metadata stored alongside the build definition.

## Envoy bazel inputs: deps and cache

The `proxyv2` envoy build has no network access: it always runs
`bazel build --nofetch`. Bazel gets two prepared inputs from git instead.

|                     | `deps` (`external.tar.gz`)      | `cache` (bazel `--disk_cache`) |
|---------------------|---------------------------------|--------------------------------|
| repository          | `istio/envoy-build-deps`        | `istio/envoy-build-cache`      |
| werf variable       | `$istioProxyDepsRev`            | `$istioProxyCacheRev`          |
| what it is          | all external repos bazel needs  | compiled build results         |
| needed for          | correctness                     | speed                          |
| if missing or stale | the build fails                 | the build is slower            |
| who makes it        | an engineer, locally (minutes)  | CI (~16 GB RAM, hours)         |

Deps are always prepared. `$buildWithPreparedCache` controls the cache:

- `true` (normal) — the cache is cloned from git to a `tmp_dir` mount, so it does not
  end up in an image layer.
- `false` — the build starts with an empty cache and keeps `/tmp/bazel-cache` in the
  artifact image, so you can export it as a new cache revision.

### Regenerating deps

Do this when the list of external repos changes: a new istio version, a new
`ENVOY_SHA`, or a patch that touches `WORKSPACE`, `go.mod` or `bazel/`. Also do it
when CI fails with `fetching repository '@…' is disabled` — the message names the
missing repo.

```bash
cd modules/110-istio/tools
go run ./build-envoy-artifacts -version 1.25.2 -artifact deps -out ~/envoy-artifacts
```

The tool runs `bazel build --nobuild` in a container (analysis only, nothing is
compiled) and writes `~/envoy-artifacts/external.tar.gz`. When it finishes it prints
the branch name to use. Commit the tarball to that branch in
`istio/envoy-build-deps` with git-lfs and update `$istioProxyDepsRev`.

Two things to check before you commit it: the archive root must be the *contents* of
`external/`, because the werf file extracts into that directory, and `local_jdk` must
not be inside, because the build image has no JDK.

Versions that build LLVM from source (1.27.9, 1.29.6) also need
`-source-repo "$SOURCE_REPO"`. That is an ssh remote, so pass a passphraseless key
with `-ssh-key ~/.ssh/id_ed25519`. Without `-ssh-key` the tool forwards
`$SSH_AUTH_SOCK`, which is what you need for a key with a passphrase.

### Regenerating the cache

Do this when the toolchain changes: another clang or LLVM version, another bazel
version, or new flags in `user.bazelrc`. Bazel hashes all of them, so an old cache
still works but never hits.

1. Set `{{- $buildWithPreparedCache := false }}` in the version's `werf.inc.yaml`.
2. Run the build in CI. It compiles everything.
3. Extract `/tmp/bazel-cache` from the `…-build-envoy-artifact` image.
4. Push it to a new branch in `istio/envoy-build-cache`, update
   `$istioProxyCacheRev`, and set `$buildWithPreparedCache` back to `true`.

Branch names look like `v<istio-version>-<envoy-commit>-<toolchain>-v<n>`. The
toolchain is part of the name because the cache depends on it. The helper prints the
whole name for you, so you only need to build it by hand for a cache made in CI.

Keep the version profiles in `modules/110-istio/tools/build-envoy-artifacts` in sync
with `$bazelVersion`, `$llvmRev`, `$llvmCacheRev` and the apt lists in these werf
files. Change them in the same commit.
