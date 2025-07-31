## How it built

### Building common-v1x19x7 and common-v1x21x6 images
  - final image based on `common/src-artifact` image
  - includes:
    - src of istio *(loaded from fox)*
    - patches in src of istio for fix healthcheck of operator
    - patches in src of istio for fix CVE

### Building cni-v1x19x7 and cni-v1x21x6 images from sources
  - final image based on `common/distroless` image
    - includes:
     - binaries install-cni *(built from src)*
     - binaries istio-cni *(built from src)*
  - build image based on `builder/golang-alpine-1.23` image
  - includes:
    - src of istio *(loaded from common-ver artifact)*
    - binaries install-cni *(built from src)*
    - binaries istio-cni *(built from src)*

### Building operator-v1x19x7 and operator-v1x21x6 images from sources
  - final image based on `common/distroless` image
    - includes:
      - binaries operator *(built from src)*
      - manifests of istio *(loaded from common-ver artifact)*
  - build image based on `builder/golang-alpine-1.23` image
    - includes:
      - src of istio *(loaded from common-ver artifact)*
      - binaries operator *(built from src)*
      - manifests of istio *(loaded from common-ver artifact)*

### Building pilot-v1x19x7 and pilot-v1x21x6 images from sources
  - final image based on `common/distroless` image
    - includes:
      - binaries pilot-discovery *(built from src)*
      - templates for envoy bootstrap *(loaded from common-ver artifact)*
  - build image based on `builder/golang-alpine-1.23` image
    - includes:
      - src of istio *(loaded from common-ver artifact)*
      - binaries pilot-discovery *(built from src)*
      - templates for envoy bootstrap *(loaded from common-ver artifact)*


### Building proxy-v1x19x7 image
  - final image based on `common/alt-p11` image
    - includes:
      - package ca-certificates from repo:p11
      - binaries iptables *(built from src)*
      - binaries pilot-agent *(built from src)*
      - templates for envoy bootstrap *(loaded from common-ver artifact)*
      - binaries envoy *(!!!from image loaded from hub.docker.com)*
  - image for build pilot-agent based on `builder/golang-alpine-1.23` image
    - includes:
        - src of istio/proxy *(loaded from fox)*
        - binaries pilot-agent *(built from src)*

### Building proxy-v1x21x6 image
  - final image based on `common/alt-p11` image
    - includes:
      - package ca-certificates from repo:p11
      - binaries iptables *(built from src)*
      - binaries pilot-agent *(built from src)*
      - templates for envoy bootstrap *(loaded from common-ver artifact)*
      - binaries envoy *(built from src, see the description below)*
  - image for build pilot-agent based on `builder/golang-alpine-1.23` image
    - includes:
        - src of istio/proxy *(loaded from fox)*
        - patches in src of istio for fix CVE
        - binaries pilot-agent *(built from src)*

#### Building envoy for proxy-v1x21x6

  - `build-image-artifact` image based on `common/alt-p11-artifact` image
    - based on `cni-cilium/base-cilium-dev` adapted for the current envoy build (e.g. llvm and bazel versions)
  - `build-libcxx-artifact` image based on `build-image-artifact` image (for build libcxxabi and libcxx)
    - This library needs for build envoy. libcxxabi and libcxx from AltLinux:P11 are not compatible with our build.
    - includes:
      - src of llvm *(loaded from fox)*
      - libraries libcxxabi and libcxx *(built from src)*
  - `build-envoy-artifact` image based on `build-image-artifact` image
    - includes:
      - src of istio/proxy *(loaded from fox)*
      - libraries libcxxabi and libcxx *(built from src)*
      - build-cache of envoy *(loaded from fox)*
      - build-deps of envoy *(loaded from fox)*
      - some patches:
        - using the self-built `libcxxabi` and `libcxxx` libraries,
        - in `WORKSPACE` we change `ENVOY_SHA` and `ENVOY_SHA256` which are links to the envoy repository version 1.29.12. Because the original tag is broken.
        - `BAZEL_LINKOPTS=-lm:-pthread` -> `BAZEL_LINKOPTS=-lm:-lpthread` in `envoy.bazelrc` *(???)*
        - use bazel build options `--config=release` and target `//:envoy`. We found this method in ProwCI in repository istio/proxy. ([Original build job from Istio ProwCI](https://prow.istio.io/view/gs/istio-prow/pr-logs/pull/istio_release-builder/1944/build-warning_release-builder_release-1.21/1837269285437706240))
      - binaries envoy *(built from src)*

