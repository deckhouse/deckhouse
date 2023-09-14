ARG BASE_ALPINE
ARG BASE_DISTROLESS

FROM $BASE_ALPINE as artifact
WORKDIR /src
ARG SOURCE_REPO

ENV SOURCE_REPO=${SOURCE_REPO}

RUN apk add coreutils curl gcc make musl-dev openssl-dev git
RUN git clone --branch 7.0.9 --depth 1 ${SOURCE_REPO}/redis/redis.git .
RUN make CFLAGS="-static" LDFLAGS="-static"

RUN chown 64535:64535 /src/src/redis-server
RUN chmod 0700 /src/src/redis-server

FROM $BASE_DISTROLESS

COPY --from=artifact /src/src/redis-server /redis-server

ENTRYPOINT ["/redis-server"]
