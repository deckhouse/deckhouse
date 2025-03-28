# syntax=docker/dockerfile:1.7-labs

FROM --platform=linux/amd64 golang:1.24 AS build

# Set workdir
WORKDIR /deckhouse

# Install setcap
RUN apt-get update && \
    apt-get install -y libcap2-bin

# Download pkg
COPY --parents **/*.mod **/*sum ./
RUN --mount=type=cache,target=/root/.cache/go-build \
        go mod download

# Build controller
COPY --parents **/*.go **/*.sh ./

ENV D8_VERSION="1.68.7-dev"
RUN --mount=type=cache,target=/root/.cache/go-build \
        cd ./deckhouse-controller/ && \
        chmod +x *.sh && \
        ./go-build.sh && \
        cp deckhouse-controller /usr/bin/deckhouse-controller && \
        cp deckhouse-controller /usr/bin/caps-deckhouse-controller && \
        setcap "cap_sys_chroot=ep cap_sys_admin=ep cap_mknod=ep" /usr/bin/caps-deckhouse-controller

# Replace controller
FROM --platform=linux/amd64 dev-registry.deckhouse.io/sys/deckhouse-oss:pr8229-ee
COPY --from=build /usr/bin/deckhouse-controller /usr/bin/deckhouse-controller
COPY --from=build /usr/bin/caps-deckhouse-controller /usr/bin/caps-deckhouse-controller

# Replace module helm chart
COPY --exclude=docs \
     --exclude=charts/helm_lib \
     --exclude=README.md \
     --exclude=images \
     --exclude=hooks/**/*.go \
     --exclude=template_tests \
     --exclude=.namespace \
     --exclude=values_matrix_test.yaml \
     --exclude=apis/**/*.go \
     --exclude=requirements/**/*.go \
     --exclude=settings-conversion/**/*.go \
     --exclude=hack/**/*.go \
     ee/modules/038-system-registry/ /deckhouse/modules/038-system-registry/
