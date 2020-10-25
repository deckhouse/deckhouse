ARG BASE_ALPINE
FROM $BASE_ALPINE
RUN apk add --no-cache bash sed curl
COPY api-proxy-check.sh /
ENTRYPOINT ["/api-proxy-check.sh"]
