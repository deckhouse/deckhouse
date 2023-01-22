ARG BASE_ALPINE
FROM quay.io/kiali/kiali:v1.62@sha256:ea49c6afb7de7cf9c1831fcd9480906c01b2322c10385a269fc2e61bd4e5fb8e as artifact

FROM $BASE_ALPINE
COPY --from=artifact /opt/kiali/ /opt/kiali/

RUN adduser -H -D -u 1000 kiali && chown -R kiali:kiali /opt/kiali/console && chmod -R g=u /opt/kiali/console

WORKDIR /opt/kiali
USER 1000

ENTRYPOINT ["/opt/kiali/kiali"]
