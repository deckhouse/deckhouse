ARG BASE_ALPINE
FROM quay.io/kiali/kiali:v1.52@sha256:cb7e5e0615ce8bb84a75e8cf10a91363a535d2c486c288800bba58885346e6f4 as artifact

FROM $BASE_ALPINE
COPY --from=artifact /opt/kiali/ /opt/kiali/

RUN adduser -H -D -u 1000 kiali && chown -R kiali:kiali /opt/kiali/console && chmod -R g=u /opt/kiali/console

WORKDIR /opt/kiali
USER 1000

ENTRYPOINT ["/opt/kiali/kiali"]
