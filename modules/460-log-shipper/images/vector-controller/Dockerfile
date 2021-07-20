FROM flant/vector:0.14.0-alpine AS vector

FROM flant/shell-operator:v1.0.1
COPY --from=vector /usr/local/bin/vector /usr/local/bin/vector
ADD vector-config-hook.sh /hooks
RUN chmod +x /hooks/vector-config-hook.sh
