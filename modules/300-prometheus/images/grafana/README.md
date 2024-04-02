---
title: How-to build grafana
---
Current grafana version is `GRAFANA_VERSION=v8.5.13`.

To build grafana we need to repush used third-party libs to own registry. To do this we need to run specific Dockerfile:

```dockerfile
ARG BASE_NODE_16_ALPINE
FROM $BASE_NODE_16_ALPINE
ARG GRAFANA_VERSION
ARG SOURCE_REPO

ENV SOURCE_REPO=${SOURCE_REPO} \
    GRAFANA_VERSION=${GRAFANA_VERSION} \
    NODE_OPTIONS="--max-old-space-size=8192"

RUN apk update && apk add git &&\
    mkdir -p /usr/src/app &&\
    cd /usr/src/app &&\
    git clone --depth 1 --branch ${GRAFANA_VERSION} ${SOURCE_REPO}/grafana/grafana.git . &&\
    yarn install
```

To run Dockerfile exec the command:

```shell
docker build --build-arg SOURCE_REPO=https://github.com --build-arg BASE_NODE_16_ALPINE=registry.deckhouse.io/base_images/node:16.13.0-alpine3.14@sha256:5277c7d171e02ee76417bb290ef488aa80e4e64572119eec0cb9fffbcffb8f6a --build-arg GRAFANA_VERSION=${GRAFANA_VERSION} -t grafana-deps .
```

Than copy folders from container:

```shell
docker run -it --name grafana-deps --entrypoint /bin/sh grafana-deps &
docker cp grafana-deps:/usr/src/app/.yarn .yarn
docker cp grafana-deps:/usr/src/app/package.json package.json
docker cp grafana-deps:/usr/src/app/yarn.lock yarn.lock
docker rm -f grafana-deps
```

Then commit content to `fox.flant.com/deckhouse/3p/grafana/grafana-deps` to the branch `${GRAFANA_VERSION}`.

## How-to build grafana-statusmap

Current grafana version is `STATUSMAP_VERSION=v0.5.1`.

To build grafana-statusmap we need to repush used third-party libs to own registry. To do this we need to run specific Dockerfile:

```dockerfile
ARG BASE_NODE_16_ALPINE
FROM $BASE_NODE_16_ALPINE
ARG STATUSMAP_VERSION
ARG SOURCE_REPO

ENV SOURCE_REPO=${SOURCE_REPO} \
    STATUSMAP_VERSION=${STATUSMAP_VERSION} \
    NODE_OPTIONS="--max-old-space-size=8192"

RUN apk update && apk add git &&\
    git clone --depth 1 --branch ${STATUSMAP_VERSION} ${SOURCE_REPO}/flant/grafana-statusmap.git /grafana-statusmap &&\
    cd /grafana-statusmap &&\
    yarn install
```

To run Dockerfile exec the command:

```shell
docker build --build-arg SOURCE_REPO=https://github.com --build-arg BASE_NODE_16_ALPINE=registry.deckhouse.io/base_images/node:16.13.0-alpine3.14@sha256:5277c7d171e02ee76417bb290ef488aa80e4e64572119eec0cb9fffbcffb8f6a --build-arg STATUSMAP_VERSION=${STATUSMAP_VERSION} -t statusmap-deps .
```

Than copy folders from container:

```shell
docker run -it --name statusmap-deps --entrypoint /bin/sh statusmap-deps &
docker cp statusmap-deps:/grafana-statusmap/node_modules node_modules
docker cp statusmap-deps:/grafana-statusmap/package.json package.json
docker rm -f statusmap-deps
```

Then commit content to `fox.flant.com/deckhouse/3p/flant/grafana-statusmap-deps` to the branch `${STATUSMAP_VERSION}`.
