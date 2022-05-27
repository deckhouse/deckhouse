ARG BASE_ALPINE
FROM $BASE_ALPINE as artifact

ARG VERSION=2.0.1
ARG COMMIT_REF=d6f0dcb883004146ca3453a9e2d0c66514afe327

RUN apk add --no-cache go git make
RUN git clone https://github.com/giantswarm/cert-exporter.git
WORKDIR /cert-exporter
RUN git checkout "${COMMIT_REF}"
RUN make

FROM $BASE_ALPINE

RUN apk add --no-cache ca-certificates
USER 1000
COPY --from=artifact /cert-exporter/cert-exporter /cert-exporter

ENTRYPOINT ["/cert-exporter"]
