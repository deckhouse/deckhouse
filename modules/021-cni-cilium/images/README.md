## How it builded

### Building BASE_CILIUM_DEV images (used for build other images and binaries, and contain all dependencies)

- `BASE_CILIUM_DEV` - contain all dependencies from original images: runtime, builder, compilers, cilium-envoy, iptables
  - based on `BASE_UBUNTU` image
  - includes `(!!! loaded from internet)`:
    - packages from repo: ubuntu-jammy and apt.llvm.org
    - binaries of go (1.21.5) from [go.dev](https://go.dev)
    - binaries and plugins of protoc (22.3) from github releases
    - binaries of bazel and wrapper shell-scripts from github releases
      - 3.7.0, 3.7.1, 3.7.2, 6.1.0

### Building utility binaries
- `+` `hubble`
  - based on `BASE_GOLANG_20_BULLSEYE_DEV` image
  - includes:
    - src of hubble *(loaded from fox)*
    - binaries hubble-cli *(builded from src)*
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
- `+` `bpftool`
  - based on `BASE_CILIUM_DEV` image
  - includes:
    - src of bpf-next *(loaded from fox)*
    - binaries bpftool *(builded from src)*
- `+` `llvm`
  - based on `BASE_CILIUM_DEV` image
  - includes:
    - src of llvm *(loaded from fox)*
    - build-cache of llvm *(loaded from fox)*
    - binaries llvm-10.0.0: clang, llc, llvm-objcopy *(builded from src)*
- `+` `iptables`
  - based on `BASE_CILIUM_DEV` image
  - includes:
    - src of iptables deb-package *(loaded from fox)*
    - deb-packages of iptables 1.8.8-1 *(builded from src)*
- `+` `cilium-envoy`
  - based on `BASE_CILIUM_DEV` image
  - includes:
    - src of cilium/proxy *(loaded from fox)*
    - src of envoyproxy/envoy *(loaded from fox)*
    - build-cache of cilium/proxy *(loaded from fox)*
    - binaries and libs of cilium-envoy *(builded from src)*
- `+` `cilium`
  - based on `BASE_CILIUM_DEV` image
  - includes:
    - binaries from image `llvm`
    - binaries from image `bpftool`
    - binaries from image `cni-plugins`
    - binaries from image `gops`
    - deb-packages loaded from image `iptables` and installed
    - src of cilium *(loaded from fox)*
    - patches
    - binaries and shell-scripts of cilium *(builded from src)*

### Building an intermediate image for combining all binary files into one place and preparing the target file system.

- `agent-binaries-artifact`
  - based on `BASE_CILIUM_DEV` image
  - includes:
    - binaries from image `hubble`
    - binaries from image `llvm`
    - binaries from image `bpftool`
    - binaries from image `cni-plugins`
    - binaries from image `gops`
    - deb-packages loaded from image `iptables` and installed
    - binaries, libs and scripts from image `cilium`
    - binaries and libs from image `cilium-envoy`
    - binaries for prepull: pause and true
    - prepared all binaries, libs and scripts what required for running cilium-agent and stored in separate dir

### Building final images (used in helm-templates)
- `agent-distroless` - the main image of cilium-agent
  - based on `distroless` image
  - includes prepared binaries, libs and scripts from image `agent-binaries-artifact`
- `operator` - the main image of cilium-operator
- `safe-agent-updater` - the image with an app that ensures the correct updating of cilium-agents
- `kube-rbac-proxy` - the image of kube-rbac-proxy modified for prepull
- `check-kernel-version` - the image of check-kernel-version modified for prepull

## How to search for target commits for image-tools

1. Cloning localy https://github.com/cilium/image-tools and go to it
2. You need to find the tag of the required image from original base dockerfiles(e.g. [here](https://github.com/cilium/cilium/blob/v1.14.5/images/runtime/Dockerfile#L8-L10)) and write it to the `IMAGE_TAG` variable, for example
   ```
   IMAGE_TAG=a8c542efc076b62ba683e7699c0013adb6955f0f
   ```
3. Find all commits corresponding to this tag, for example
   ```
   git rev-list --all | git diff-tree --stdin --find-object=$IMAGE_TAG | grep -B1 -E "^:(\b\w+\b\s){3}$IMAGE_TAG"
   ```
4. And select the most recent one (by date)

## Original Building Container Images

In general, the original description is [here](https://github.com/cilium/cilium/blob/v1.14.5/Documentation/contributing/development/images.rst) and [here](https://docs.cilium.io/en/v1.14/contributing/development/images/), but it may not be accurate

### At the time of writing the instructions, the dependency was something like this:

**Building utility images (used for build other images and binaries)**
- `compilers`
  - based on `UBUNTU`
  - includes:
    - deb-packages from ubuntu package repository
    - bazel
- `cilium-envoy-builder`
  - based on `UBUNTU`
  - includes:
    - deb-packages from ubuntu package repository
    - deb-packages from apt.llvm.org
    - go and bazel
- `runtime`
  - based on `UBUNTU`
  - includes:
    - binaries from image `gops-cni`
    - binaries from image `llvm`
    - binaries from image `bpftool`
    - binaries from image `iptables`
    - deb-packages from ubuntu package repository
- `builder`
  - based on `runtime` image
  - includes:
    - binaries from image `llvm`
    - deb-packages from ubuntu package repository
    - go, protoc and bazel

**Building utility binaries**
- `cilium-envoy`
  - based on `cilium-envoy-builder` image
  - includes:
    - binaries of cilium-envoy *(builded from src)*
- `llvm`
  - based on `compilers` image
  - includes:
    - binaries llvm-10.0.0 *(builded from src)*
- `bpftool`
  - based on `compilers` image
  - includes:
    - binaries bpftool *(builded from src)*
- `iptables`
  - based on `UBUNTU` image
  - includes:
    - deb-packages from ubuntu package repository
    - deb-packages from debiad package repository
    - deb-packages iptables 1.8.8-1 *(builded from src)*
- `gops-cni`
  - based on `GO` image
  - includes:
    - binaries of gops *(builded from src)*
    - binaries of cni-plugins *(loaded from internet)*
- `hubble`
  - based on `builder` image
  - includes:
    - binaries hubble-cli *(builded from src)*
- `cilium-builder`
  - based on `builder` image
  - includes:
    - binaries, libs and scripts of cilium *(builded from src)*

**Building final images (used in helm-templates)**
- `cilium`
  - based on `runtime` image
  - includes:
    - binaries, libs scripts and from image `cilium-builder`
    - binaries from image `cilium-envoy`
    - binaries from image `hubble`

## original_build_way -> our_build_way

- All dependencies are collected in one image (`BASE_CILIUM_DEV`)
- All "non-common" binaries and packages build from sources
- *todo* All common packages taken from ALTLinux
- All utility images are based on `BASE_CILIUM_DEV` and are build in one pass (without complex multi-stage assemblies)
- Final image based on distroless
