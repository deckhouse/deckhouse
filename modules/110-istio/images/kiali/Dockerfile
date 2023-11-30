ARG BASE_ALPINE

# original kiali
FROM quay.io/kiali/kiali:v1.76.1@sha256:44e702107f9a7c718b08c9cbeb303cc75974a2590fc38459fcddae5efa2b0e39 as artifact

FROM $BASE_ALPINE
# get all from original kiali
COPY --from=artifact /opt/kiali/ /opt/kiali/

RUN adduser -H -D -u 1000 kiali && chown -R kiali:kiali /opt/kiali/console && chmod -R g=u /opt/kiali/console

WORKDIR /opt/kiali
USER 1000

ENTRYPOINT ["/opt/kiali/kiali"]

CMD ["kiali"]
