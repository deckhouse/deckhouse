ARG BASE_ALPINE
ARG BASE_GOLANG_18_ALPINE
ARG VERSION=1.62.0

# build kiali backend with patch
FROM $BASE_GOLANG_18_ALPINE as builder
ARG VERSION

ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /src

RUN apk add --no-cache git && \
    git clone --branch v${VERSION} --depth 1 https://github.com/kiali/kiali.git .

RUN go mod download

COPY patches/001-fix-fetching-istiod-resource-threholds.patch .
RUN git apply 001-fix-fetching-istiod-resource-threholds.patch

RUN go build \
      -v \
      -ldflags "-w -s -X main.version=${VERSION} -X main.commitHash='deckhouse'" \
      -o kiali .

# original kiali
FROM quay.io/kiali/kiali:v1.62.0@sha256:ea49c6afb7de7cf9c1831fcd9480906c01b2322c10385a269fc2e61bd4e5fb8e as artifact

FROM $BASE_ALPINE
# get all from original kiali
COPY --from=artifact /opt/kiali/ /opt/kiali/
# replace kiali with patched version
COPY --from=builder /src/kiali /opt/kiali/kiali

RUN adduser -H -D -u 1000 kiali && chown -R kiali:kiali /opt/kiali/console && chmod -R g=u /opt/kiali/console

WORKDIR /opt/kiali
USER 1000

ENTRYPOINT ["/opt/kiali/kiali"]

CMD ["kiali"]
