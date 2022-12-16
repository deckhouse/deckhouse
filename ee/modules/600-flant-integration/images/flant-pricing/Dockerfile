ARG BASE_SHELL_OPERATOR
FROM $BASE_SHELL_OPERATOR
COPY hooks/ /hooks
RUN apk add --no-cache curl
RUN apk add --no-cache micropython --repository http://dl-3.alpinelinux.org/alpine/edge/testing/
