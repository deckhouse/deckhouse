ARG BASE_ALPINE
FROM $BASE_ALPINE
RUN apk add --no-cache bash sed curl jq
COPY api-proxy-check.sh nodeport-bind-address.sh /
