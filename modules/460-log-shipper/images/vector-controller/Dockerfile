ARG BASE_SHELL_OPERATOR
FROM flant/vector:0.16.1-alpine@sha256:c0705a557fcf9022c0b23a50d6eb1d51d33207310c61dd58106aab9b23f77ebb AS vector

FROM $BASE_SHELL_OPERATOR
COPY --from=vector /usr/local/bin/vector /usr/local/bin/vector
ADD vector-config-hook.sh /hooks
RUN chmod +x /hooks/vector-config-hook.sh
