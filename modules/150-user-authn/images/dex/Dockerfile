# Based on https://github.com/dexidp/dex/blob/v2.28.1/Dockerfile
ARG BASE_GOLANG_ALPINE
ARG BASE_ALPINE
FROM $BASE_GOLANG_ALPINE as artifact
RUN apk add --no-cache git ca-certificates gcc build-base sqlite patch make curl
WORKDIR /dex
COPY client-groups.patch obsolete.patch static-user-groups.patch /
RUN wget https://github.com/dexidp/dex/archive/v2.28.1.tar.gz -O - | tar -xz --strip-components=1 \
  && git apply /client-groups.patch \
  && git apply /static-user-groups.patch \
  && git apply /obsolete.patch
RUN go build ./cmd/dex

FROM dexidp/dex:v2.28.1@sha256:5e88f2205de172b60fd7af23ac92f34321688a83de9f7de7c9a6f394f6950877 as dex

FROM $BASE_ALPINE
RUN apk add --no-cache --update ca-certificates openssl
RUN mkdir -p /var/dex
RUN chown -R 1001:1001 /var/dex
RUN mkdir -p /etc/dex
RUN chown -R 1001:1001 /etc/dex
# Copy module files for CVE scanning / dependency analysis.
COPY --from=dex /etc/dex/config.docker.yaml /etc/dex/
COPY --from=dex /usr/local/src/dex/go.mod /usr/local/src/dex/go.sum /usr/local/src/dex/
COPY --from=dex /usr/local/src/dex/api/v2/go.mod /usr/local/src/dex/api/v2/go.sum /usr/local/src/dex/api/v2/

COPY --from=dex /usr/local/bin/gomplate /usr/local/bin/
COPY --from=dex /entrypoint.sh /

COPY --from=artifact /dex/dex /usr/local/bin/
COPY --from=artifact /dex/web /web

USER 1001:1001

ENTRYPOINT ["/entrypoint.sh"]
CMD ["dex", "serve", "/etc/dex/config.docker.yaml"]
