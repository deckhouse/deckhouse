---
title: How-to build grafana
---
Current grafana version is `GRAFANA_VERSION=v10.4.10`.

To build grafana we need to repush used third-party libs to own registry. To do this we need to run specific Dockerfile:

```dockerfile
ARG BASE_NODE_20_ALPINE
FROM $BASE_NODE_20_ALPINE
ARG GRAFANA_VERSION
ARG SOURCE_REPO

ENV SOURCE_REPO=${SOURCE_REPO} \
    GRAFANA_VERSION=${GRAFANA_VERSION} \
    NODE_OPTIONS="--max-old-space-size=8192"

RUN apk update && apk add git base-build make python3 &&\
    git clone --depth 1 --branch ${GRAFANA_VERSION} ${SOURCE_REPO}/grafana/grafana.git /src &&\
    cd /src &&\
    yarn install --immutable &&\
    export NODE_ENV=production NODE_OPTIONS="--max_old_space_size=8000" &&\
    yarn build
```

To run Dockerfile exec the command:

```shell
docker build --build-arg SOURCE_REPO=https://github.com --build-arg BASE_NODE_20_ALPINE="registry.deckhouse.ru/base_images/node:20.11.0-alpine3.18@sha256:bd2eb17dcdc3541d4986bebcfc997a24c499358827899b1029af3601d4c4569d" --build-arg GRAFANA_VERSION=${GRAFANA_VERSION} -t grafana-deps .
```

Than copy folders from container:

```shell
docker run -it --name grafana-deps --entrypoint /bin/sh grafana-deps &
docker cp grafana-deps:/src/public public
docker rm -f grafana-deps
```

Then commit content to `fox.flant.com/deckhouse/3p/grafana/grafana-deps` to the branch `${GRAFANA_VERSION}`.
