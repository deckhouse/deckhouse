ARG BASE_SHELL_OPERATOR
FROM flant/vector:0.15.1-alpine@sha256:1f6cf8d5f50f05b19686eea3ca97be5c7117e24ae86f69997e46748254747ce2 AS vector

FROM $BASE_SHELL_OPERATOR
COPY --from=vector /usr/local/bin/vector /usr/local/bin/vector
ADD vector-config-hook.sh /hooks
RUN chmod +x /hooks/vector-config-hook.sh
