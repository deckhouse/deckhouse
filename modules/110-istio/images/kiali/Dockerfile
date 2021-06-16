ARG BASE_ALPINE
FROM quay.io/kiali/kiali:v1.34@sha256:31286129a1c6f25275517ae4c243c1cde8c39b022c2c7e0b328648c639bcfb03 as artifact

FROM $BASE_ALPINE
COPY --from=artifact /opt/kiali/ /opt/kiali/

RUN adduser -H -D -u 1000 kiali && chown -R kiali:kiali /opt/kiali/console && chmod -R g=u /opt/kiali/console

WORKDIR /opt/kiali
USER 1000

ENTRYPOINT ["/opt/kiali/kiali"]
