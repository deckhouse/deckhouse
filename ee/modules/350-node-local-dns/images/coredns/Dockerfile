# Based on https://github.com/coredns/coredns/blob/master/Dockerfile
ARG BASE_ALPINE
FROM coredns/coredns:1.6.9@sha256:40ee1b708e20e3a6b8e04ccd8b6b3dd8fd25343eab27c37154946f232649ae21 as artifact

ARG BASE_ALPINE
FROM $BASE_ALPINE
COPY --from=artifact /coredns  /coredns
COPY start.sh /
COPY readiness.sh /
COPY liveness.sh /
RUN apk add --no-cache bind-tools bash curl iptables iproute2 jq grep
RUN mkdir /etc/coredns
ENTRYPOINT ["/coredns"]
