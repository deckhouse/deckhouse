ARG BASE_GOLANG_19_ALPINE
ARG BASE_DISTROLESS

FROM $BASE_GOLANG_19_ALPINE as artifact

WORKDIR /src/
COPY exporter/ /src/
RUN apk add --no-cache git && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o loop main.go

FROM $BASE_DISTROLESS

COPY --from=artifact /src/loop /

ENTRYPOINT ["/loop"]
