ARG BASE_SHELL_OPERATOR
FROM flant/vector:0.15.4-alpine@sha256:78cd243e9a8ebc279a8cc8586398fe0b12f089db60ac62c90d39910e4057ea7f AS vector

FROM $BASE_SHELL_OPERATOR
COPY --from=vector /usr/local/bin/vector /usr/local/bin/vector
ADD vector-config-hook.sh /hooks
RUN chmod +x /hooks/vector-config-hook.sh
