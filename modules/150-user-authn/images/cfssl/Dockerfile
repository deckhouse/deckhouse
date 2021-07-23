ARG BASE_ALPINE
FROM $BASE_ALPINE

RUN apk update && \
  apk add --no-cache bash jq curl && \
  apk add --no-cache --repository=http://nl.alpinelinux.org/alpine/edge/community/ --repository=http://nl.alpinelinux.org/alpine/edge/testing/ jo && \
  curl -L https://github.com/cloudflare/cfssl/releases/download/v1.6.0/cfssl_1.6.0_linux_amd64 -o /usr/local/bin/cfssl && \
  chmod +x /usr/local/bin/cfssl
