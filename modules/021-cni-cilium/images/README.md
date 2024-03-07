## How it builded

### Build utility binaries
- `+` `gops`
  - based on `BASE_GOLANG_20_ALPINE` image
  - includes:
    - installed packages from repo: binutils git
    - src of gops
    - binaries of gops (builded from src)
- `+` `cni-plugins`
  - based on `BASE_GOLANG_21_ALPINE` image
  - includes:
    - installed packages from repo: binutils git
    - src of cni-plugins
    - binaries of cni-plugins (builded from src)
- `+` `hubble`
  - based on `BASE_GOLANG_20_ALPINE` image
  - includes:
    - installed packages from repo: binutils git make
    - src of hubble
    - binaries hubble-cli (builded from src)
- *todo* `bpftool`
  - based on `compilers` image
  - includes:
  ```
  - ?? libelf1
  ```
- *todo* `llvm`
  - based on `compilers` image
  - includes:
- *todo* `iptables`
  - based on `ubuntu:22.04` image
  - includes:
    - installed packages from repo: debian-archive-keyring apt-src ca-certificates
    - loaded from repo src of iptables 1.8.8-1 and builded deb-package
- *todo* `cilium-envoy`
- `+` `cilium`
  - based on `builder` image
  - includes:
    - src of cilium
    - patches
    - binaries of cilium (builded from src)
    - shell-scripts from cilium src: init-container.sh install-plugin.sh cni-uninstall.sh


### Build utility images (used for for build other images and binaries)
- `runtime`
  - based on `BASE_UBUNTU` image
  - includes:
    - binaries from image `llvm`
    - binaries from image `bpftool`
    - binaries from image `cni-plugins`
    - binaries from image `gops`
    - binaries from image `cni-plugins`
    - binaries from image `iptables`
    - shell-scripts from cilium src: iptables-wrapper-installer.sh
    - installed packages from repo: bash-completion iproute2 iptables ipset kmod ca-certificates
    - installed packages from iptables-binaries and shell-scripts
- `builder`
  - based on `runtime` image
  - includes:
    - binaries from image `llvm`
    - binaries from image `BASE_GOLANG_20_BULLSEYE`
    - installed packages from repo: gcc g++ libc6-dev binutils coreutils curl gcc libc6-dev git make patch unzip
    - binaries and plugins of protoc `(!!! loaded from internet)`
  ```
  - ?? libelf1, libmnl0
  - ?? WORKDIR /go/src/github.com/cilium/cilium
  - ?? protoc 22.3
  ```
- `compilers`
  - based on `BASE_UBUNTU` image
  - includes:
    - installed packages from repo
    - binaries of bazel and wrapper shell-scripts `(!!! loaded from internet)`


### Build final images (used in helm-templates)
- `agent` - the main image of cilium-agent
  - based on `runtime` image
  - includes:
    - binaries from image `cilium`
    - shell-scripts from image `cilium`: init-container.sh install-plugin.sh cni-uninstall.sh
    - binaries from image `cilium-envoy`
    - binaries from image `hubble`
- `operator` - the main image of cilium-operator
- `safe-agent-updater` - the image with an app that ensures the correct updating of cilium-agents
- `kube-rbac-proxy` - the image of kube-rbac-proxy modified for prepull
- `check-kernel-version` - the image of check-kernel-version modified for prepull

