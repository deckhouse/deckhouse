ARG BASE_DISTROLESS
FROM quay.io/prometheus-operator/prometheus-operator:v0.62.0@sha256:83974739969e6f5f1672d7efb3fb710717beda26a5b69f1f9b9d89a8d4f00bcf as artifact

FROM $BASE_DISTROLESS

COPY --from=artifact /bin/operator /bin/

RUN chown nobody:nogroup /bin/operator

USER nobody

ENTRYPOINT ["/bin/operator"]
