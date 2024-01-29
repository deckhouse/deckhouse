---
title: How-to build grafana
---

Current grafana version is `GRAFANA_VERSION=8.5.13`.

To build grafana we need to repush used third-party libs to own registry. To do this we need to run specific Dockerfile:

```dockerfile
ARG BASE_NODE_16_ALPINE
FROM $BASE_NODE_16_ALPINE
ARG GRAFANA_VERSION
ARG SOURCE_REPO

ENV SOURCE_REPO=${SOURCE_REPO} \
    GRAFANA_VERSION=${GRAFANA_VERSION}

RUN apk update && apk add git &&\
    mkdir -p /usr/src/app &&\
    cd /usr/src/app &&\
    git clone --depth 1 --branch v${GRAFANA_VERSION} ${SOURCE_REPO}/grafana/grafana.git . &&\
    yarn install
```

To run Dockerfile exec the command:

```shell
docker build --build-arg SOURCE_REPO=https://github.com --build-arg BASE_NODE_16_ALPINE=registry.deckhouse.io/base_images/node:16.13.0-alpine3.14@sha256:5277c7d171e02ee76417bb290ef488aa80e4e64572119eec0cb9fffbcffb8f6a --build-arg GRAFANA_VERSION=${GRAFANA_VERSION} -t grafana-deps .
```

Than copy folders from container:

```shell
docker run -d --name grafana-deps --entrypoint /bin/sh grafana-deps
docker cp grafana-deps:/usr/src/app/.yarn/cache cache
docker cp grafana-deps:/usr/src/app/.yarn/cache package-lock.json
docker rm -f grafana-deps
```

Then commit content to `fox.flant.com/deckhouse/3p/grafana/grafana-deps` to the branch `${GRAFANA_VERSION}`.
