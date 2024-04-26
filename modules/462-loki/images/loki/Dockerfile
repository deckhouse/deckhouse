ARG BASE_GOLANG_20_ALPINE_DEV
ARG BASE_DISTROLESS

FROM $BASE_GOLANG_20_ALPINE_DEV as artifact

ARG SOURCE_REPO
ARG GOPROXY

ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    GOOS=linux \
    GOARCH=amd64 \
    CGO_ENABLED=0

RUN git clone --depth 1 --branch v2.9.7 ${SOURCE_REPO}/grafana/loki.git /loki
WORKDIR /loki/

RUN go get golang.org/x/net@v0.17.0 \
    && go mod tidy \
    && go mod vendor
RUN go build -ldflags="-s -w" -o loki cmd/loki/main.go && \
    chown -R 64535:64535 /loki/ && \
    chmod 0700 /loki/loki

FROM $BASE_DISTROLESS
COPY --from=artifact /loki/loki /usr/bin
WORKDIR /loki/

EXPOSE 3100

ENTRYPOINT ["/usr/bin/loki"]
