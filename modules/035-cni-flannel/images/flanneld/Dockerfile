FROM quay.io/coreos/flannel:v0.13.1-rc2-amd64
COPY entrypoint.sh /
RUN apk add --no-cache curl jq

# https://github.com/coreos/flannel/issues/1002 workaround
STOPSIGNAL SIGKILL

ENTRYPOINT ["/entrypoint.sh"]
