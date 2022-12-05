ARG BASE_ALPINE
FROM quay.io/kiali/kiali:v1.49@sha256:ed80930d2d6b3e435399062825a69f8ffa78653cd5059518982503f0d20c20da as artifact

FROM $BASE_ALPINE
COPY --from=artifact /opt/kiali/ /opt/kiali/

RUN adduser -H -D -u 1000 kiali && chown -R kiali:kiali /opt/kiali/console && chmod -R g=u /opt/kiali/console

WORKDIR /opt/kiali
USER 1000

ENTRYPOINT ["/opt/kiali/kiali"]
