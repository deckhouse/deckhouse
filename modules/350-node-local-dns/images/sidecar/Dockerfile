FROM flant/shell-operator:v1.0.0-beta.5-alpine3.9
COPY common/ /
COPY init/ /
COPY hooks/ /hooks
RUN apk add --no-cache iptables bash iproute2 curl jq
