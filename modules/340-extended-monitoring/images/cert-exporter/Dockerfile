ARG BASE_DISTROLESS
ARG BASE_GOLANG_16_ALPINE

FROM $BASE_GOLANG_16_ALPINE as artifact

ARG VERSION=2.0.1
ARG COMMIT_REF=d6f0dcb883004146ca3453a9e2d0c66514afe327

ARG SOURCE_REPO
ENV SOURCE_REPO=${SOURCE_REPO}

ARG GOPROXY
ENV GOPROXY=${GOPROXY}

RUN apk add --no-cache go git make openssh-client
RUN mkdir -p ~/.ssh && echo "StrictHostKeyChecking accept-new" > ~/.ssh/config
RUN --mount=type=ssh git clone --depth 1 --branch v2.0.1 ${SOURCE_REPO}/giantswarm/cert-exporter.git /cert-exporter

WORKDIR /cert-exporter
RUN make && \
    chown -R 64535:64535 /cert-exporter/ && \
    chmod 0700 /cert-exporter/cert-exporter
FROM $BASE_DISTROLESS

COPY --from=artifact /cert-exporter/cert-exporter /cert-exporter

ENTRYPOINT ["/cert-exporter"]
