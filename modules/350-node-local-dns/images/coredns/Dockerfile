FROM alpine:3.11.2
COPY --from=gcr.io/google-containers/coredns:1.6.6 /coredns  /coredns
COPY start.sh /
COPY liveness.sh /
COPY stop.sh /
RUN apk add --no-cache bind-tools bash curl iptables iproute2 jq grep lockfile-progs
RUN mkdir /etc/coredns
ENTRYPOINT ["/coredns"]
