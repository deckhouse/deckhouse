ARG BASE_DISTROLESS
ARG BASE_ALPINE

# original kiali
FROM quay.io/kiali/kiali:v1.67.2@sha256:92fec2b44580519b922d37747114865df4cc4cd51a2fe8342f090c8941e6905d as artifact

FROM $BASE_ALPINE as artifact2
# get all from original kiali
COPY --from=artifact /opt/kiali/ /opt/kiali/

RUN adduser -H -D -u 1000 kiali && chown -R kiali:kiali /opt/kiali/console && chmod -R g=u /opt/kiali/console

FROM $BASE_DISTROLESS
# get all from original kiali
COPY --from=artifact2 /opt/kiali/ /opt/kiali/

WORKDIR /opt/kiali
USER 1000

ENTRYPOINT ["/opt/kiali/kiali"]

CMD ["kiali"]
