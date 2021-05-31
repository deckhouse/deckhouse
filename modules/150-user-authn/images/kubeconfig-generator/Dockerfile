ARG BASE_ALPINE
ARG BASE_GOLANG_ALPINE
FROM $BASE_GOLANG_ALPINE

RUN apk add bash git make

ENV GO111MODULE=on

WORKDIR /app

ADD already_logged.patch /

RUN git clone https://github.com/mintel/dex-k8s-authenticator.git . && \
  git checkout 378a39dd93bed9f56a5a1b1a799a208c61ead83f && \
  git apply --whitespace=fix /already_logged.patch && \
  go mod download && \
  make build

FROM $BASE_ALPINE

RUN apk add --update ca-certificates openssl curl tini

RUN mkdir -p /app/bin
COPY --from=0 /app/bin/dex-k8s-authenticator /app/bin/
COPY --from=0 /app/html /app/html
COPY --from=0 /app/templates /app/templates
COPY --from=0 /app/entrypoint.sh /

RUN mkdir -p /certs && chmod a+x /entrypoint.sh

WORKDIR /app

ENTRYPOINT ["/sbin/tini", "--", "/entrypoint.sh"]

CMD ["--help"]
