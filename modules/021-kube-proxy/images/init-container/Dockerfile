ARG BASE_DISTROLESS
ARG BASE_GOLANG_19_ALPINE
FROM $BASE_GOLANG_19_ALPINE as artifact

WORKDIR /src/
COPY entrypoint/ /src/
RUN apk add --no-cache git && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o entrypoint main.go

FROM $BASE_DISTROLESS

COPY --from=artifact /src/entrypoint /
ENTRYPOINT [ "/entrypoint" ]
