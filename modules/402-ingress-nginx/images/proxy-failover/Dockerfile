ARG BASE_NGINX_ALPINE
ARG BASE_GOLANG_19_ALPINE

FROM $BASE_GOLANG_19_ALPINE as artifact
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -ldflags='-extldflags "-static" -s -w' -o entrypoint main.go

FROM $BASE_NGINX_ALPINE
RUN rm -rf /etc/nginx

COPY rootfs /
COPY --from=artifact /src/entrypoint /

ENTRYPOINT ["/entrypoint"]
CMD ["/usr/sbin/nginx", "-g", "daemon off;"]
