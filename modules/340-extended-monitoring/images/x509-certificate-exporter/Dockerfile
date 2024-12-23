ARG BASE_DISTROLESS
ARG BASE_GOLANG_23_ALPINE

FROM $BASE_GOLANG_23_ALPINE as artifact

ARG VERSION=3.15.0
ARG SOURCE_REPO
ARG GOPROXY

ENV SOURCE_REPO=${SOURCE_REPO}
ENV GOPROXY=${GOPROXY}

WORKDIR /x509-certificate-exporter
RUN apk add --no-cache go git make openssh-client && \
    mkdir -p ~/.ssh && echo "StrictHostKeyChecking accept-new" > ~/.ssh/config
RUN --mount=type=ssh git clone --depth 1 --branch v${VERSION} ${SOURCE_REPO}/enix/x509-certificate-exporter.git . && \
    go build -v -tags netgo,osusergo ./cmd/x509-certificate-exporter && \
    chown -R 64535:64535 /x509-certificate-exporter/ && \
    chmod 0700 /x509-certificate-exporter/x509-certificate-exporter

FROM $BASE_DISTROLESS
COPY --from=artifact /x509-certificate-exporter/x509-certificate-exporter /x509-certificate-exporter
ENTRYPOINT [ "/x509-certificate-exporter" ]
