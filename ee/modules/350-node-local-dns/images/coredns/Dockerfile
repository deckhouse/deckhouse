# Based on https://github.com/coredns/coredns/blob/master/Dockerfile
ARG BASE_ALPINE
FROM coredns/coredns:1.9.3@sha256:bdb36ee882c13135669cfc2bb91c808a33926ad1a411fee07bd2dc344bb8f782 as artifact

ARG BASE_ALPINE
FROM $BASE_ALPINE
COPY --from=artifact /coredns  /coredns
COPY start.sh /
COPY readiness.sh /
COPY liveness.sh /
RUN apk add --no-cache bind-tools bash curl iptables iproute2 jq grep
RUN mkdir /etc/coredns
ENTRYPOINT ["/coredns"]
