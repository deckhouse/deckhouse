ARG BASE_GOLANG_19_ALPINE
ARG COMMON/DISTROLESS_IMAGE_NAME

FROM $BASE_GOLANG_19_ALPINE as artifact

WORKDIR /src/
COPY exporter/ /src/
RUN apk add --no-cache git && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o loop main.go

FROM $COMMON/DISTROLESS_IMAGE_NAME

COPY --from=artifact /src/loop /
ENTRYPOINT ["/loop"]
