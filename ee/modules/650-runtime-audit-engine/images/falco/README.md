---
title: "How-to build falco"
---

Current falco version is `FALCO_VERSION=0.35.1`.

To build falco we need to repush used third-party libs to own registry. To do this we need to run specific Dockerfile:

```dockerfile
ARG BASE_ALT_DEV
FROM $BASE_ALT_DEV
ARG FALCO_VERSION
ARG SOURCE_REPO

ENV SOURCE_REPO=${SOURCE_REPO} \
    FALCO_VERSION=${FALCO_VERSION}
    
RUN git clone --branch ${FALCO_VERSION} --depth 1 ${SOURCE_REPO}/falcosecurity/falco.git && \
    mkdir -p /falco/build && \
    cd /falco/build && \
    rm -f /usr/bin/clang && \
    ln -s /usr/bin/clang-15 /usr/bin/clang && \
    cmake -DCMAKE_BUILD_TYPE=release -DCMAKE_INSTALL_PREFIX=/usr -DBUILD_DRIVER=OFF -DBUILD_BPF=OFF -DBUILD_FALCO_MODERN_BPF=ON -DBUILD_WARNINGS_AS_ERRORS=OFF -DFALCO_VERSION="${FALCO_VERSION}" -DUSE_BUNDLED_DEPS=ON /falco && \
    sed -i "s/DEB;RPM;TGZ/TGZ/" ./CPackConfig.cmake && \
    make package -j4 && \
    mkdir /falco-deps && \
    rm -rf _CPack_Packages && \
    rm -f falco*.deb falco*.tar.gz falco*.rpm falco*.tar.gz && \
    find . -type f -name "*.tar.gz" -exec rsync -avR \{} /falco-deps \; && \
    find . -type f -name "*.tar.bz2" -exec rsync -avR \{} /falco-deps \; && \
    cd grpc-prefix/src/grpc && \
    make clean && \
    cd /falco/build && \
    tar -czvf /falco-deps/grpc.tar.gz grpc-prefix/src
```

To run Dockerfile exec the command:

```shell
docker build --build-arg SOURCE_REPO=https://github.com --build-arg BASE_ALT_DEV=registry.deckhouse.io/base_images/dev-alt:p10 --build-arg FALCO_VERSION=${FALCO_VERSION} -t falco-deps .
```

Than copy folder `/falco-deps` from container:

```shell
docker run --name falco-deps -d falco-deps bash
docker cp falco-deps:/falco-deps falco-deps
docker rm -f falco-deps
```

Then commit content of falco-deps to `fox.flant.com/deckhouse/3p/falcosecurity/falco-deps` to the branch `${FALCO_VERSION}`.
