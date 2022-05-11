ARG BASE_UBUNTU
FROM docker.io/envoyproxy/envoy:v1.18.4@sha256:e5c2bb2870d0e59ce917a5100311813b4ede96ce4eb0c6bfa879e3fbe3e83935 as artifact

FROM $BASE_UBUNTU
COPY --from=artifact /usr/local/bin/envoy /usr/local/bin/envoy
COPY --from=artifact /etc/envoy /etc/envoy
RUN chown nobody /usr/local/bin/envoy
RUN chmod +x /usr/local/bin/envoy

USER nobody
CMD ["envoy", "-c", "/etc/envoy/envoy.yaml"]
