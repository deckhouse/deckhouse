FROM alpine:3.12
RUN apk add --no-cache bash sed curl
COPY api-proxy-check.sh /
ENTRYPOINT ["/api-proxy-check.sh"]
