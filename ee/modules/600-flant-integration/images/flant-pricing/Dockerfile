ARG BASE_SHELL_OPERATOR
FROM $BASE_SHELL_OPERATOR
COPY hooks/ /hooks
COPY requirements.txt /tmp/requirements.txt
RUN apk add --no-cache curl python3 py3-pip && \
  pip3 install "$(grep shell /tmp/requirements.txt)" && \
  apk del py3-pip && \
  rm -rf /var/cache/apk/*
