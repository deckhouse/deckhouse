# Based on https://github.com/coredns/coredns/blob/master/Dockerfile
ARG BASE_ALPINE
FROM coredns/coredns:1.9.1@sha256:d5a7db9ab4cb3efc22a08707385c54c328db3df32841d6c4a8ae78f102f1f49a as artifact

ARG BASE_ALPINE
FROM $BASE_ALPINE
COPY --from=artifact /coredns  /coredns
COPY start.sh /
COPY readiness.sh /
COPY liveness.sh /
RUN apk add --no-cache bind-tools bash curl iptables iproute2 jq grep
RUN mkdir /etc/coredns
ENTRYPOINT ["/coredns"]
