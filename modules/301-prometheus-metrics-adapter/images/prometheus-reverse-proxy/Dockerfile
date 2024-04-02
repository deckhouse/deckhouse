ARG BASE_DISTROLESS
ARG BASE_GOLANG_20_ALPINE

FROM $BASE_GOLANG_20_ALPINE as artifact
ARG GOPROXY
ENV GOPROXY=${GOPROXY}
WORKDIR /src/
COPY /app/ /src/
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test ./... && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" ./cmd/prometheus-reverse-proxy

RUN chown 64535:64535 prometheus-reverse-proxy
RUN chmod 0700 prometheus-reverse-proxy

FROM $BASE_DISTROLESS
COPY --from=artifact /src/prometheus-reverse-proxy /prometheus-reverse-proxy
ENTRYPOINT [ "/prometheus-reverse-proxy" ]
