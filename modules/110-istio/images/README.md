## How it built

### Building common-v1x21x6 images
  - final image based on `common/src-artifact` image
  - includes:
    - src of istio *(loaded from fox)*
    - patches in src of istio for fix healthcheck of operator
    - patches in src of istio for fix CVE

### Building cni-v1x21x6 images from sources
  - final image based on `common/distroless` image
    - includes:
     - binaries install-cni *(built from src)*
     - binaries istio-cni *(built from src)*
  - build image based on `builder/golang-alpine-1.23` image
  - includes:
    - src of istio *(loaded from common-ver artifact)*
    - binaries install-cni *(built from src)*
    - binaries istio-cni *(built from src)*

### Building operator-v1x21x6 images from sources
  - final image based on `common/distroless` image
    - includes:
      - binaries operator *(built from src)*
      - manifests of istio *(loaded from common-ver artifact)*
  - build image based on `builder/golang-alpine-1.23` image
    - includes:
      - src of istio *(loaded from common-ver artifact)*
      - binaries operator *(built from src)*
      - manifests of istio *(loaded from common-ver artifact)*

### Building pilot-v1x21x6 images from sources
  - final image based on `common/distroless` image
    - includes:
      - binaries pilot-discovery *(built from src)*
      - templates for envoy bootstrap *(loaded from common-ver artifact)*
  - build image based on `builder/golang-alpine-1.23` image
    - includes:
      - src of istio *(loaded from common-ver artifact)*
      - binaries pilot-discovery *(built from src)*
      - templates for envoy bootstrap *(loaded from common-ver artifact)*

### Building proxy-v1x21x6 image
  - final image based on `common/distroless` image
    - includes:
      - `libc.so.6`, `libm.so.6` and `ld-linux-x86-64.so.2` *(from the `libs/glibc-v2.41` image)*
      - binaries iptables *(from registrypackages)*
      - binaries pilot-agent *(built from src)*
      - templates for envoy bootstrap *(loaded from common-ver artifact)*
      - binaries envoy *(built from src, see the description below)*
  - image for build pilot-agent based on `builder/golang` image
    - includes:
        - src of istio *(loaded from common-ver artifact)*
        - binaries pilot-agent *(built from src)*

#### Building envoy for proxy-v1x21x6

  - `build-image-artifact` image based on `builder/golang-bookworm` image
    - clang/lld 14 and libc++/libc++abi 14 come from Debian bookworm packages — the same
      toolchain layout as proxy-v1x25x2 and proxy-v1x27x9
    - protoc, the protoc-gen-* plugins and bazel are fetched from internal mirrors
  - `build-envoy-artifact` image based on `build-image-artifact` image
    - includes:
      - src of istio/proxy *(loaded from fox)*
      - build-cache of envoy *(loaded from fox)*
      - build-deps of envoy *(loaded from fox)*
      - some patches:
        - in `WORKSPACE` we change `ENVOY_SHA` and `ENVOY_SHA256` which are links to the envoy repository version 1.29.12. Because the original tag is broken.
        - use bazel build options `--config=release` and target `//:envoy`. We found this method in ProwCI in repository istio/proxy. ([Original build job from Istio ProwCI](https://prow.istio.io/view/gs/istio-prow/pr-logs/pull/istio_release-builder/1944/build-warning_release-builder_release-1.21/1837269285437706240))
      - binaries envoy *(built from src)*

  Envoy is linked against the builder's glibc (2.36 on bookworm) and libc++ is linked
  statically, so the final distroless image only needs the three glibc files listed above.
  The `$buildWithPreparedCacheAndDeps` variable controls whether the bazel disk cache and
  the pre-fetched `external` tree are pulled from `istio/envoy-build-cache` and
  `istio/envoy-build-deps`; set it to `false` to produce a fresh cache/deps pair, which is
  required whenever the build toolchain changes.

