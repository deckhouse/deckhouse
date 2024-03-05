---
title: "How-to build ingress nginx"
---

Current vector version is `CONTROLLER_VERSION=controller-v1.9.5`.

To build ingress-nginx we need to repush used third-party libraries to own registry. To do this we need to run specific Dockerfile:

```dockerfile
ARG BASE_ALT_DEV
FROM $BASE_ALT_DEV
ARG CONTROLLER_VERSION
ARG SOURCE_REPO

ENV SOURCE_REPO=${SOURCE_REPO} \
    CONTROLLER_VERSION=${VECTOR_VERSION}
    
RUN git clone --depth 1 --branch ${CONTROLLER_VERSION} ${SOURCE_REPO}/kubernetes/ingress-nginx.git /src &&\
    cd /src/images/nginx/rootfs/ &&\
    ./build.sh
```

To run Dockerfile exec the command:

```shell
docker build --build-arg SOURCE_REPO=https://github.com --build-arg BASE_ALT_DEV=registry.deckhouse.io/base_images/dev-alt:p10 --build-arg CONTROLLER_VERSION=${CONTROLLER_VERSION} -t ingress-nginx-jaegertracing-deps .
```

Than copy folder `/root/.hunter` from container:

```shell
docker run --name ingress-nginx-jaegertracing-deps -d vector-deps bash
docker cp ingress-nginx-jaegertracing-deps:/root/.hunter ingress-nginx-jaegertracing-deps
docker rm -f ingress-nginx-jaegertracing-deps
```

Then commit content of ingress-nginx-jaegertracing-deps to `fox.flant.com/deckhouse/3p/kubernetes/ingress-nginx-jaegertracing-deps` to the branch `${CONTROLLER_VERSION}`.
