# syntax=docker/dockerfile:1.7-labs

FROM --platform=linux/amd64 golang:1.24 AS src

RUN apt-get update && \
    apt-get install -y rsync

COPY . /deckhouse

RUN mkdir -p /artifacts/mod && \
  rsync -a --include '*/' --include='go.mod' --include='go.sum' --exclude='*' --prune-empty-dirs /deckhouse/ /artifacts/mod/

RUN mkdir -p /artifacts/src && \
  rsync -a --include '*/' --include='*.go' --include='deckhouse-controller/*.sh' --exclude='*' --prune-empty-dirs /deckhouse/ /artifacts/src/

RUN mkdir -p /artifacts/registry /artifacts/deckhouse && \
    rsync -a --prune-empty-dirs \
      --exclude='docs' \
      --exclude='charts/helm_lib' \
      --exclude='README.md' \
      --exclude='images' \
      --exclude='hooks/**.go' \
      --exclude='template_tests' \
      --exclude='.namespace' \
      --exclude='values_matrix_test.yaml' \
      --exclude='apis/**/*.go' \
      --exclude='requirements/**/*.go' \
      --exclude='settings-conversion/**/*.go' \
      --exclude='hack/**/*.go' \
      --exclude='.dmtlint.yaml' \
      /deckhouse/modules/038-registry/ /artifacts/registry/ && \
    rsync -a --prune-empty-dirs \
      --exclude='docs' \
      --exclude='charts/helm_lib' \
      --exclude='README.md' \
      --exclude='images' \
      --exclude='hooks/**.go' \
      --exclude='template_tests' \
      --exclude='.namespace' \
      --exclude='values_matrix_test.yaml' \
      --exclude='apis/**/*.go' \
      --exclude='requirements/**/*.go' \
      --exclude='settings-conversion/**/*.go' \
      --exclude='hack/**/*.go' \
      --exclude='.dmtlint.yaml' \
      /deckhouse/modules/002-deckhouse/ /artifacts/deckhouse/

RUN mkdir -p /out/run && \
    cd /out && \
    chmod 755 /run

FROM --platform=linux/amd64 golang:1.24 AS build

# Set workdir
WORKDIR /deckhouse

# Install setcap
RUN apt-get update && \
    apt-get install -y libcap2-bin

RUN mkdir -p /deckhouse
WORKDIR /deckhouse

# Download pkg
COPY --from=src /artifacts/mod .

RUN --mount=type=cache,target=/root/.cache/go-build \
        go mod download -x

# Build controller
COPY --from=src /artifacts/src .

ENV D8_VERSION="1.68.7-dev"
ENV DEFAULT_KUBERNETES_VERSION="1.30"
RUN --mount=type=cache,target=/root/.cache/go-build \
        cd ./deckhouse-controller/ && \
        chmod +x *.sh && \
        ./go-build.sh
RUN mkdir -p /out/ && \
    cp /deckhouse/deckhouse-controller/deckhouse-controller /out/deckhouse-controller && \
    cp /deckhouse/deckhouse-controller/deckhouse-controller /out/caps-deckhouse-controller && \
    setcap "cap_sys_chroot=ep cap_sys_admin=ep cap_mknod=ep" /out/caps-deckhouse-controller

# Replace controller
FROM --platform=linux/amd64 dev-registry.deckhouse.io/sys/deckhouse-oss:pr14860

COPY --from=build /out/deckhouse-controller /usr/bin/deckhouse-controller
COPY --from=build /out/caps-deckhouse-controller /usr/bin/caps-deckhouse-controller
COPY --from=src /out/run /run

USER root
# RUN ["/bin/bash", "-c", "cp /bin/bash /bin/sh"]

# Replace module helm chart
RUN rm -r \
  /deckhouse/modules/038-registry/templates \
  /deckhouse/modules/038-registry/openapi \
  /deckhouse/modules/038-registry/monitoring \
  /deckhouse/modules/002-deckhouse/templates \
  /deckhouse/modules/002-deckhouse/openapi

USER deckhouse

COPY --from=src /artifacts/registry /deckhouse/modules/038-registry
COPY --from=src /artifacts/deckhouse /deckhouse/modules/002-deckhouse