ARG BASE_DEBIAN
FROM $BASE_DEBIAN as builder
ARG CUBERITE_GITREPO=https://github.com/cuberite/cuberite.git
ARG CUBERITE_COMMIT_REF=de5b89dbea938db5a7ea58f59a9b1a91d0df4208

RUN apt-get update \
 && apt-get install -y clang cmake git make python3 \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

RUN git clone --recursive ${CUBERITE_GITREPO} /tmp/cuberite \
    && cd /tmp/cuberite \
    && git checkout ${CUBERITE_COMMIT_REF} \
    && cmake . -DCMAKE_BUILD_TYPE=RELEASE -DWHOLE_PROGRAM_OPTIMISATION=0 \
    && make -j 2

FROM $BASE_DEBIAN
COPY --from=builder /tmp/cuberite/Server /opt/cuberite
RUN mkdir -p /etc/cuberite /opt/cuberite

EXPOSE 8080 25565
WORKDIR /opt/cuberite

ENTRYPOINT ["/opt/cuberite/Cuberite"]
