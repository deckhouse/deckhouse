---
title: "How-to build falco"
---

Current vector version is `VECTOR_VERSION=0.31.0`.

To build vector we need to repush used third-party rust libraries to own registry. To do this we need to run specific Dockerfile:

```dockerfile
ARG BASE_ALT_DEV
FROM $BASE_ALT_DEV
ARG VECTOR_VERSION
ARG SOURCE_REPO

ENV SOURCE_REPO=${SOURCE_REPO} \
    VECTOR_VERSION=${VECTOR_VERSION}
    
RUN source "$HOME/.cargo/env" &&\
    git clone --depth 1 --branch v${VECTOR_VERSION} https://github.com/vectordotdev/vector.git &&\
    cd /vector &&\
    cargo vendor
```

To run Dockerfile exec the command:

```shell
docker build --build-arg SOURCE_REPO=https://github.com --build-arg BASE_ALT_DEV=registry.deckhouse.io/base_images/dev-alt:p10 --build-arg VECTOR_VERSION=${VECTOR_VERSION} -t vector-deps .
```

Than copy folder `/vector/vendor` from container:

```shell
docker run --name vector-deps -d vector-deps bash
docker cp vector-deps:/vector/vendor vector-deps
docker rm -f vector-deps
```

Then commit content of vector-deps to `fox.flant.com/deckhouse/3p/vectordotdev/vector-deps` to the branch `${VECTOR_VERSION}`.
