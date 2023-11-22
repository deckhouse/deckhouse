ARG BASE_ALPINE

# original kiali
FROM quay.io/kiali/kiali:v1.75.1@sha256:cb656ca6b06c1d0f44a7a2cbb47f8a17b8ba2e3c80ae4b6a9f1e1bbe4dae668c as artifact

FROM $BASE_ALPINE
# get all from original kiali
COPY --from=artifact /opt/kiali/ /opt/kiali/

RUN adduser -H -D -u 1000 kiali && chown -R kiali:kiali /opt/kiali/console && chmod -R g=u /opt/kiali/console

WORKDIR /opt/kiali
USER 1000

ENTRYPOINT ["/opt/kiali/kiali"]

CMD ["kiali"]
