# Based on https://github.com/dexidp/dex/blob/v2.35.3/Dockerfile
ARG BASE_GOLANG_18_ALPINE
ARG BASE_ALPINE
FROM $BASE_GOLANG_18_ALPINE as artifact
RUN apk add --no-cache git ca-certificates gcc build-base sqlite patch make curl
WORKDIR /dex
COPY patches/client-groups.patch patches/static-user-groups.patch patches/gitlab-refresh-context.patch patches/connector-data.patch patches/oidc-ca-insecure.patch /
RUN wget https://github.com/dexidp/dex/archive/v2.35.3.tar.gz -O - | tar -xz --strip-components=1 \
  && git apply /client-groups.patch \
  && git apply /static-user-groups.patch \
  && git apply /gitlab-refresh-context.patch \
  && git apply /connector-data.patch \
  && git apply /oidc-ca-insecure.patch
RUN go build ./cmd/dex

FROM $BASE_ALPINE
RUN apk add --no-cache --update ca-certificates openssl
RUN mkdir -p /var/dex
RUN chown -R 1001:1001 /var/dex
RUN mkdir -p /etc/dex
RUN chown -R 1001:1001 /etc/dex

COPY --from=artifact /dex/dex /usr/local/bin/
COPY --from=artifact /dex/web /web

USER 1001:1001

CMD ["dex", "serve", "/etc/dex/config.docker.yaml"]
