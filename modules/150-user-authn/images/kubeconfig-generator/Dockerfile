ARG BASE_ALPINE
ARG BASE_GOLANG_ALPINE
FROM $BASE_GOLANG_ALPINE as artifact

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
COPY --from=artifact /app/bin/dex-k8s-authenticator /app/bin/
COPY --from=artifact /app/html /app/html
COPY --from=artifact /app/templates /app/templates
COPY --from=artifact /app/entrypoint.sh /

RUN mkdir -p /certs \
  # set up nsswitch.conf for Go's "netgo" implementation
  # Go stdlib completely ignores /etc/hosts file without it
  # https://github.com/moby/moby/issues/34544
  && echo "hosts: files dns" > /etc/nsswitch.conf \
  && chmod a+x /entrypoint.sh

WORKDIR /app

ENTRYPOINT ["/sbin/tini", "--", "/entrypoint.sh"]

CMD ["--help"]
