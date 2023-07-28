ARG BASE_DISTROLESS
ARG BASE_GOLANG_16_ALPINE

FROM $BASE_GOLANG_16_ALPINE as artifact

ARG VERSION=2.0.1
ARG COMMIT_REF=d6f0dcb883004146ca3453a9e2d0c66514afe327

RUN apk add --no-cache go git make
RUN git clone --depth 1 --branch v2.0.1 https://github.com/giantswarm/cert-exporter.git /cert-exporter

WORKDIR /cert-exporter
RUN make

FROM $BASE_DISTROLESS

USER 1000
COPY --from=artifact /cert-exporter/cert-exporter /cert-exporter

ENTRYPOINT ["/cert-exporter"]
