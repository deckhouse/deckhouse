ARG BASE_NGINX_ALPINE
FROM flant/kube-ca-auth-proxy:v0.5.6@sha256:2ed8e7573049cd8eb9f22d4f243017668b79a4968254e0c4c0fe3ee67729e836 as artifact

FROM $BASE_NGINX_ALPINE
RUN apk add openssl --update && \
    rm -rf /var/cache/apk/* && \
    rm -rf /etc/nginx/* && \
    mkdir /etc/nginx/certs

COPY --from=artifact /bin/run-proxy /bin/run-proxy

ENTRYPOINT ["/bin/run-proxy"]
