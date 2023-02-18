ARG BASE_ALPINE
FROM prom/alertmanager:v0.25.0@sha256:db8303fa05341f5dc6b19b36a97325cd1b8307254ed9042a2c554af71f3c0284 as artifact

FROM $BASE_ALPINE
COPY --from=artifact /bin/alertmanager /bin

USER nobody
ENTRYPOINT ["/bin/alertmanager"]
