ARG BASE_GOLANG_19_ALPINE_DEV
ARG BASE_DISTROLESS

FROM $BASE_GOLANG_19_ALPINE_DEV as artifact

ARG GOPROXY
ARG SOURCE_REPO

ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /src/
COPY exporter/ /src/
RUN go get golang.org/x/net@v0.17.0 \
    && go mod tidy \
    && go build -ldflags="-s -w" -o loop main.go && \
    chown -R 64535:64535 /src/ && \
    chmod 0700 /src/loop

FROM $BASE_DISTROLESS

COPY --from=artifact /src/loop /

ENTRYPOINT ["/loop"]
