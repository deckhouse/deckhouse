ARG BASE_SHELL_OPERATOR
FROM timberio/vector:0.20.0-alpine@sha256:0b88b7f3a77d0374ecc36023dcf139379ca6ee1e4883c002e9bdada3e641af70 AS vector

FROM $BASE_SHELL_OPERATOR
RUN apk add --no-cache gettext
COPY --from=vector /usr/local/bin/vector /usr/local/bin/vector
ADD vector-config-hook.sh /hooks
RUN chmod +x /hooks/vector-config-hook.sh
