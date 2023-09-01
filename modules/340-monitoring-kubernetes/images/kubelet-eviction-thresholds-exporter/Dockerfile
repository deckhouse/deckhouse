ARG BASE_GOLANG_19_ALPINE
ARG BASE_DISTROLESS

FROM $BASE_GOLANG_19_ALPINE as artifact

ARG GOPROXY
ENV GOPROXY=${GOPROXY}

WORKDIR /src/
COPY exporter/ /src/
RUN apk add --no-cache git && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o loop main.go && \
    chown -R 64535:64535 /src/ && \
    chmod 0700 /src/loop

FROM $BASE_DISTROLESS

COPY --from=artifact /src/loop /

ENTRYPOINT ["/loop"]
