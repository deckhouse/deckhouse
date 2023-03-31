ARG BASE_ALPINE
ARG BASE_GOLANG_19_ALPINE


FROM $BASE_GOLANG_19_ALPINE as artifact

WORKDIR /src/
COPY main.go go.mod go.sum /src/
RUN apk add --no-cache git && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o loop main.go

FROM scratch

COPY --from=artifact /src/loop /
ENTRYPOINT ["/loop"]
