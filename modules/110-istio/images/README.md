## How it built

### Building cni-v1x19x7 and cni-v1x21x6 images from sources
  - final image based on `common/distroless` image
  - build image based on `BASE_GOLANG_23_ALPINE` image
  - includes:
    - src of istio *(loaded from fox)*
    - binaries install-cni *(built from src)*
    - binaries istio-cni *(built from src)*

### Building operator-v1x19x7 and operator-v1x21x6 images from sources
  - final image based on `common/distroless` image
  - build image based on `BASE_GOLANG_23_ALPINE` image
  - includes:
    - src of istio *(loaded from fox)*
    - binaries operator *(built from src)*
    - manifests from src of istio *(loaded from fox)*

### Building pilot-v1x19x7 and pilot-v1x21x6 images from sources
  - final image based on `common/distroless` image
  - build image based on `BASE_GOLANG_23_ALPINE` image
  - includes:
    - src of istio *(loaded from fox)*
    - binaries pilot-discovery *(built from src)*
    - templates for envoy bootstrap from src of istio *(loaded from fox)*

### Building common-v1x19x7 and common-v1x21x6 images
  - final image based on `common/src-artifact` image
  - includes:
    - src of istio *(loaded from fox)*
    - patches in src of istio for fix CVE and healthcheck of operator

### Building pilot-v1x19x7 image
  - final image based on `common/alt-p11` image
    - includes:
        - templates for envoy bootstrap from src of istio *(loaded from fox)*
        - binaries pilot-discovery *(built from src)*
        - binaries iptables and etc *(relocate from binaries-artifact)*
        - binaries envoy *(relocate from istio proxyv2 image, now we not builded it)*
  - build image based on `BASE_GOLANG_23_ALPINE` image with name `{{ .ModuleName }}/{{ .ImageName }}-binary-artifact` for build pilot-agent
    - includes:
        - src of istio *(loaded from fox)*
        - binaries pilot-agent *(built from src)*
  - relocate image based on `common/relocate-artifact` image with name `{{ .ModuleName }}/{{ .ImageName }}-binaries-artifact` for to prepare iptables and their binaries
    - includes:
        - binaries iptables iptables-save iptables-restore ip6tables ip6tables-save ip6tables-restore

### Building pilot-v1x21x6 image
  - final image based on `common/alt-p11` image
    - includes:
        - templates for envoy bootstrap from src of istio *(loaded from fox)*
        - binaries pilot-discovery *(built from src)*
        - binaries iptables and etc *(relocate from binaries-artifact)*
        - binaries envoy *(built from src, See the description below)*
  - build image based on `BASE_GOLANG_23_ALPINE` image with name `{{ .ModuleName }}/{{ .ImageName }}-binary-artifact` for build pilot-agent
    - includes:
        - src of istio *(loaded from fox)*
        - binaries pilot-agent *(built from src)*
  - relocate image based on `common/relocate-artifact` image with name `{{ .ModuleName }}/{{ .ImageName }}-binaries-artifact` for to prepare iptables and their binaries
    - includes:
        - binaries iptables iptables-save iptables-restore ip6tables ip6tables-save ip6tables-restore
  - build image based on `common/alt-p11-artifact` image with name `{{ .ModuleName }}/{{ .ImageName }}-build-image-artifact` for build in this image envoy source code
    - includes:
        - see `werf.inc.yaml` (The image is based on a similar image from cni-cilium named base-cilium-dev. The image has been adapted for the current envoy build.)
  - build image based on `{{ $.ModuleName }}/{{ .ImageName }}-build-image-artifact` image with name `{{ .ModuleName }}/{{ .ImageName }}-build-libcxx-artifact` for build in this image libcxxabi and libcxx. This library needs for build envoy. libcxxabi and libcxx from Alt linux we can't use them because they are built with another linker (ld.lld linked to alt-wraper) that can't be used to build envoy.
    - includes:
        - see `werf.inc.yaml`
  - build image based on `{{ $.ModuleName }}/{{ .ImageName }}-build-image-artifact` image with name `{{ .ModuleName }}/{{ .ImageName }}-build-envoy-artifact` for build envoy binary
    - includes:
        - src of istio/proxy *(loaded from fox)*
        - library libcxxabi and libcxx *(loaded from image {{ .ModuleName }}/{{ .ImageName }}-build-libcxx-artifact )*
        - cache for build bazel *(loaded from fox)* Cache moved after success build envoy to fox (see README.md how create cache). Cache needed for quick build envoy in bazel without build all dependency.
        -  deps for build bazel *(loaded from fox)* Deps moved after success build envoy to fox (see README.md how create deps). Deps needed for quick build envoy in bazel without download all dependency.
    Build envoy in bazel contains a some of edits in envoy.bazelrc and WORKSPACE for the correct build of envoy in Alt linux (see `werf.inc.yaml`). In WORKSPACE we change ENVOY_SHA and ENVOY_SHA256 which are links to the envoy repository version 1.29.12. In envoy.bazelrc we added fix to specify the correct path to the libcxx libraries and use lpthread, not pthread.
    Bazel always run with options --config=release with target //:envoy because it's correct build with stripe. This meathod found in source repository istio/proxy in folder prow (it's CI) and in CI [web interface](https://prow.istio.io/view/gs/istio-prow/pr-logs/pull/istio_release-builder/1944/build-warning_release-builder_release-1.21/1837269285437706240) istio.
