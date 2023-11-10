# Based on https://github.com/falcosecurity/falcosidekick/blob/41d530807f1a0294c0276e4cb42af68c8b26a659/Dockerfile
ARG BASE_GOLANG_20_ALPINE
ARG BASE_DISTROLESS

FROM $BASE_GOLANG_20_ALPINE as artifact
ARG GOPROXY
ARG SOURCE_REPO

ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /src
RUN apk add --no-cache make git bash && \
    git clone -c advice.detachedHead=false --branch 2.28.0 --depth 1 ${SOURCE_REPO}/falcosecurity/falcosidekick.git . && \
    go get -u github.com/nats-io/nkeys@v0.4.6 && \
    go get -u golang.org/x/net@v0.17.0 && \
    go get -u google.golang.org/grpc@v1.56.3 && \
    make falcosidekick && \
    chown -R 64535:64535 /src/falcosidekick && \
    chmod 0755 /src/falcosidekick

FROM $BASE_DISTROLESS
COPY --from=artifact /src/falcosidekick /falcosidekick
ENTRYPOINT [ "/falcosidekick" ]
