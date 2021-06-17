ARG BASE_ALPINE
ARG BASE_GOLANG_ALPINE

FROM $BASE_GOLANG_ALPINE as artifact
WORKDIR /go/src/github.com/discordianfish/nginx_exporter
RUN apk add --no-cache git \
  && mkdir -p /go/src/github.com/nginxinc/ \
  && cd /go/src/github.com/nginxinc/ \
  && git clone --branch v0.8.0 --depth 1 https://github.com/nginxinc/nginx-prometheus-exporter.git
RUN cd /go/src/github.com/nginxinc/nginx-prometheus-exporter \
  && GO111MODULE=on CGO_ENABLED=0 GOOS=linux go build -mod=vendor -a -installsuffix cgo -ldflags "-X main.version=0.8.0 -X main.gitCommit=f0173677183c840e90a56e48082e36ac687e1a30" -o exporter .

FROM $BASE_ALPINE
COPY --from=artifact /go/src/github.com/nginxinc/nginx-prometheus-exporter/exporter /usr/bin/
ENTRYPOINT ["/usr/bin/exporter"]
