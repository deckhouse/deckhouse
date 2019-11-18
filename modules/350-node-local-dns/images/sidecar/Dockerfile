FROM flant/shell-operator:v1.0.0-beta.5-alpine3.9
COPY hooks/ /hooks
RUN apk add --no-cache netcat-openbsd bind-tools iptables bash iproute2 curl jq grep && wget -O /usr/local/bin/yq "https://github.com/mikefarah/yq/releases/download/2.4.0/yq_linux_amd64" && chmod +x /usr/local/bin/yq
