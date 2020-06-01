FROM alpine:3.10

RUN apk update && \
  apk add bash jq && \
  apk add --repository=http://nl.alpinelinux.org/alpine/edge/community/ --repository=http://nl.alpinelinux.org/alpine/edge/testing/ jo cfssl
