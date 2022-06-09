ARG BASE_ALPINE
FROM prom/alertmanager:v0.24.0@sha256:b1ba90841a82ea24d79d4e6255b96025a9e89275bec0fae87d75a5959461971e as artifact

FROM $BASE_ALPINE
COPY --from=artifact /bin/alertmanager /bin

USER nobody
ENTRYPOINT ["/bin/alertmanager"]
