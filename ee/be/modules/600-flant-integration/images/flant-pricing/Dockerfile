ARG BASE_SHELL_OPERATOR
FROM $BASE_SHELL_OPERATOR
COPY requirements.txt /tmp/requirements.txt
RUN apk add --no-cache curl python3 py3-pip && \
  pip3 install --break-system-packages -r /tmp/requirements.txt && \
  apk del py3-pip && \
  rm -rf /var/cache/apk/*
COPY hooks/ /hooks
