ARG BASE_ALPINE
ARG BASE_GOLANG_ALPINE
FROM $BASE_GOLANG_ALPINE

RUN apk add --no-cache --update alpine-sdk bash patch git

ENV GO111MODULE=on

WORKDIR /app

ADD already_logged.patch /

RUN git clone -b v1.2.0 https://github.com/mintel/dex-k8s-authenticator.git . && \
  patch -p1 < /already_logged.patch && \
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
