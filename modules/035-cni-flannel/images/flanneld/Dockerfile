# Based on https://github.com/flannel-io/flannel/blob/master/Dockerfile.amd64
ARG BASE_ALPINE
FROM quay.io/coreos/flannel:v0.15.1-amd64@sha256:a3ebdc7e5e44d1ba3ba8ccd8399e81444102bd35f5f480997a637a42d1e1da6b as base

FROM $BASE_ALPINE

COPY --from=base /opt/bin/flanneld /opt/bin/
COPY --from=base /opt/bin/mk-docker-opts.sh /opt/bin/

COPY entrypoint.sh /
COPY iptables-wrapper-installer.sh /
RUN apk add --no-cache curl jq
RUN apk add --no-cache iproute2 net-tools ca-certificates iptables ip6tables conntrack-tools strongswan && update-ca-certificates
RUN apk add wireguard-tools --no-cache --repository http://dl-cdn.alpinelinux.org/alpine/edge/community
RUN /iptables-wrapper-installer.sh --no-sanity-check

# https://github.com/coreos/flannel/issues/1002 workaround
STOPSIGNAL SIGKILL

ENTRYPOINT ["/entrypoint.sh"]
