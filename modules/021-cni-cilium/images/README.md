## How it builded

### Building utility binaries
- `+` `gops`
  - based on `BASE_GOLANG_21_ALPINE_DEV` image
  - includes:
    - src of gops *(loaded from fox)*
    - binaries of gops *(builded from src)*
- `+` `cni-plugins`
  - based on `BASE_GOLANG_21_ALPINE_DEV` image
  - includes:
    - src of cni-plugins *(loaded from fox)*
    - binaries of cni-plugins *(builded from src)*
- `+` `hubble`
  - based on `BASE_GOLANG_20_BULLSEYE_DEV` image
  - includes:
    - src of hubble *(loaded from fox)*
    - binaries hubble-cli *(builded from src)*
- `+` `bpftool`
  - based on `BASE_CILIUM_DEV` image
  - includes:
    - src of bpf-next *(loaded from fox)*
    - binaries bpftool *(builded from src)*
- `+` `llvm`
  - based on `BASE_CILIUM_DEV` image
  - includes:
    - src of llvm *(loaded from fox)*
    - binaries llvm-10.0.0: clang, llc, llvm-objcopy *(builded from src)*
- `+` `cilium`
  - based on `BASE_CILIUM_DEV` image
  - includes:
    - binaries from image `llvm`
    - binaries from image `bpftool`
    - binaries from image `cni-plugins`
    - binaries from image `gops`
    - deb-package from image `iptables`
    - installed packages from image `iptables`
    - src of cilium *(loaded from fox)*
    - patches
    - binaries and shell-scripts of cilium *(builded from src)*
- *todo* `cilium-envoy`
  - based on `BASE_UBUNTU` image
  - includes:
    - installed packages from repo `(!!! loaded from internet)`
    - binaries of bazel(6.1.0) `(!!! loaded from internet)`
    - src of envoyproxy/envoy *(loaded from fox)*
    - src of cilium/proxy *(loaded from fox)*
    - binaries cilium-envoy *(builded from src)*
- *todo* `iptables`
  - based on `ubuntu:22.04` image
  - includes:
    - installed packages from repo `(!!! loaded from internet)`:
      - debian-archive-keyring apt-src ca-certificates
    - loaded src-deb-package from repo `(!!! loaded from internet)`:
      - iptables 1.8.8-1
    - rebuilded deb-package

### Building utility images (used for build other images and binaries)

- `BASE_CILIUM_DEV`
  - contain all dependances from original images
    - runtime
    - builder
    - compilers
  - based on `BASE_UBUNTU` image
  - includes:
    - installed packages from repo `(!!! loaded from internet)`
    - binaries of go (1.21.5) `(!!! loaded from internet)`
    - binaries of bazel and wrapper shell-scripts `(!!! loaded from internet)`
      - 3.7.0, 3.7.1, 3.7.2, 6.1.0
    - binaries and plugins of protoc (22.3) `(!!! loaded from internet)`

- `agent-binaries-artifact`
  - based on `BASE_CILIUM_DEV` image
  - includes:
    - binaries from image `llvm`
    - binaries from image `bpftool`
    - binaries from image `cni-plugins`
    - binaries from image `gops`
    - binaries, libs and scripts from image `cilium`
    - binaries and libs from image `cilium-envoy`
    - binaries from image `hubble`
    - deb-package from image `iptables`
    - installed packages from image `iptables`
    - shell-scripts from cilium src: iptables-wrapper-installer.sh
    - prepared all binaries, libs and scripts what required for running cilium-agent

  ```
    - `builder`
      - ?? libelf1, libmnl0
      - ?? WORKDIR /go/src/github.com/cilium/cilium
      - ?? protoc 22.3
  ```

### Building final images (used in helm-templates)
- `agent-distroless` - the main image of cilium-agent
  - based on `distroless` image
  - includes prepared binaries, libs and scripts from image `agent-binaries-artifact`
- `operator` - the main image of cilium-operator
- `safe-agent-updater` - the image with an app that ensures the correct updating of cilium-agents
- `kube-rbac-proxy` - the image of kube-rbac-proxy modified for prepull
- `check-kernel-version` - the image of check-kernel-version modified for prepull

