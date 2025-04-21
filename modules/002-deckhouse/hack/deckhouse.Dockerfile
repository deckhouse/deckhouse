# syntax=docker/dockerfile:1.7-labs

FROM --platform=linux/amd64 golang:1.24 AS src

RUN apt-get update && \
    apt-get install -y rsync

COPY . /deckhouse

RUN mkdir -p /artifacts/mod && \
  rsync -a --include '*/' --include='go.mod' --include='go.sum' --exclude='*' --prune-empty-dirs /deckhouse/ /artifacts/mod/

RUN mkdir -p /artifacts/src && \
  rsync -a --include '*/' --include='*.go' --include='deckhouse-controller/*.sh' --exclude='*' --prune-empty-dirs /deckhouse/ /artifacts/src/

RUN mkdir -p /artifacts/registry && \
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
      /deckhouse/ee/modules/038-system-registry/ /artifacts/registry/


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


# Replace controller
FROM --platform=linux/amd64 dev-registry.deckhouse.io/sys/deckhouse-oss:pr8229-ee

COPY --from=build /deckhouse/deckhouse-controller/deckhouse-controller /usr/bin/deckhouse-controller

USER root

RUN cp /usr/bin/deckhouse-controller /usr/bin/caps-deckhouse-controller && \
    setcap "cap_sys_chroot=ep cap_sys_admin=ep cap_mknod=ep" /usr/bin/caps-deckhouse-controller

USER deckhouse


# Replace module helm chart
RUN rm -r \
  /deckhouse/modules/038-system-registry/templates \
  /deckhouse/modules/038-system-registry/openapi

COPY --from=src /artifacts/registry /deckhouse/modules/038-system-registry
