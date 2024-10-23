ARG BASE_ALT
ARG BASE_GOLANG_16_ALPINE
FROM $BASE_GOLANG_16_ALPINE as artifact
WORKDIR /src/
COPY main.go go.mod go.sum /src/
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o metadata-exporter main.go

FROM $BASE_ALT
RUN apt-get update && \
  apt-get install -y \
  curl \
  && apt-get clean \
  && rm -rf  /var/log/*log /var/lib/apt/lists/* /var/log/apt/* /var/lib/dpkg/*-old /var/cache/debconf/*-old
COPY --from=artifact /src/metadata-exporter /metadata-exporter
ENTRYPOINT [ "/metadata-exporter" ]
