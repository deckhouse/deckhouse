ARG BASE_SHELL_OPERATOR
FROM flant/vector:0.14.1-alpine@sha256:563f5dde722f70724837d51341afbaefb953bf3c1d725cb28ef660fe7bb5c1d5 AS vector

FROM $BASE_SHELL_OPERATOR
RUN apk add --no-cache gettext
COPY --from=vector /usr/local/bin/vector /usr/local/bin/vector
ADD vector-config-hook.sh /hooks
RUN chmod +x /hooks/vector-config-hook.sh
