ARG BASE_GOLANG_20_ALPINE
ARG BASE_DISTROLESS

FROM $BASE_GOLANG_20_ALPINE as artifact

ARG SOURCE_REPO
ARG GOPROXY

ENV SOURCE_REPO=${SOURCE_REPO} \
    GOOS=linux \
    GOARCH=amd64 \
    CGO_ENABLED=0

RUN apk add --no-cache make git
RUN git clone --depth 1 --branch v2.7.3 ${SOURCE_REPO}/grafana/loki.git /loki
WORKDIR /loki/
RUN go mod edit -go 1.20
RUN go get -u google.golang.org/grpc@v1.59.0
RUN go get -u golang.org/x/net@v0.17.0 \
    && go mod tidy \
    && go mod vendor \
    && go build -ldflags="-s -w" -o loki cmd/loki/main.go && \
    chown -R 64535:64535 /loki/ && \
    chmod 0700 /loki/loki

FROM $BASE_DISTROLESS
COPY --from=artifact /loki/loki /usr/bin
WORKDIR /loki/

EXPOSE 3100

ENTRYPOINT ["/usr/bin/loki"]
