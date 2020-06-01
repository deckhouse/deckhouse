FROM golang:1.9-alpine3.7
WORKDIR /go/src/github.com/discordianfish/nginx_exporter
RUN apk add --no-cache git && git clone https://github.com/discordianfish/nginx_exporter.git . && git checkout v0.1.0
COPY nginx-exporter.patch /
RUN patch -p1 < /nginx-exporter.patch && go get -d && go build

FROM alpine:3.7
COPY --from=0 /go/src/github.com/discordianfish/nginx_exporter/nginx_exporter /bin/
USER nobody
ENTRYPOINT [ "/bin/nginx_exporter" ]
