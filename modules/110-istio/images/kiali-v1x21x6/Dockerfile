ARG BASE_ALT

# original kiali
FROM quay.io/kiali/kiali:v1.81.0@sha256:c3d71c47a8f970b896b6cbf92718276bfc7205037f90155cdd37ee2fb57e8962 as artifact

FROM $BASE_ALT
# get all from original kiali
COPY --from=artifact /opt/kiali/ /opt/kiali/

RUN adduser --no-create-home --uid 1000 kiali && chown -R kiali:kiali /opt/kiali/console && chmod -R g=u /opt/kiali/console

WORKDIR /opt/kiali
USER 1000

ENTRYPOINT ["/opt/kiali/kiali"]

CMD ["kiali"]
