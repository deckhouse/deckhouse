ARG BASE_SHELL_OPERATOR
FROM $BASE_SHELL_OPERATOR
COPY hooks/ /hooks
RUN apk add --no-cache curl python3 && \
  rm -rf /var/cache/apk/*
