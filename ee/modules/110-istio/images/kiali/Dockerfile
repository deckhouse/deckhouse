ARG BASE_ALPINE
FROM quay.io/kiali/kiali:v1.56@sha256:5d63cb0af6a4eb2a1af012bac7688db39851aa8e1532728156081c94da1bfdd8 as artifact

FROM $BASE_ALPINE
COPY --from=artifact /opt/kiali/ /opt/kiali/

RUN adduser -H -D -u 1000 kiali && chown -R kiali:kiali /opt/kiali/console && chmod -R g=u /opt/kiali/console

WORKDIR /opt/kiali
USER 1000

ENTRYPOINT ["/opt/kiali/kiali"]
