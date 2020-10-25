ARG BASE_ALPINE
ARG BASE_GOLANG_ALPINE
FROM $BASE_GOLANG_ALPINE
WORKDIR /go/src/github.com/discordianfish/nginx_exporter
RUN apk add --no-cache git patch && git clone https://github.com/discordianfish/nginx_exporter.git . && git checkout v0.1.0
COPY nginx-exporter.patch /
RUN patch -p0 < /nginx-exporter.patch && go get -d && go build

FROM $BASE_ALPINE
COPY --from=0 /go/src/github.com/discordianfish/nginx_exporter/nginx_exporter /bin/
USER nobody
ENTRYPOINT [ "/bin/nginx_exporter" ]
