ARG BASE_SHELL_OPERATOR
FROM $BASE_SHELL_OPERATOR
COPY hooks/ /hooks
RUN apk add --no-cache curl python3 py3-pip && \
  pip3 install shell-operator==0.0.4 && \
  apk del py3-pip && \
  rm -rf /var/cache/apk/*
