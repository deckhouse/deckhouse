# Based on https://github.com/dexidp/dex/blob/v2.35.3/Dockerfile
ARG BASE_GOLANG_18_ALPINE
ARG BASE_DISTROLESS
ARG SOURCE_REPO
FROM $BASE_GOLANG_18_ALPINE as artifact
ENV SOURCE_REPO=${SOURCE_REPO}
RUN apk add --no-cache git ca-certificates gcc build-base sqlite patch make curl
WORKDIR /dex
COPY patches/client-groups.patch patches/static-user-groups.patch patches/gitlab-refresh-context.patch patches/connector-data.patch patches/oidc-ca-insecure.patch patches/robots-txt.patch /
RUN git clone --branch v2.35.3 --depth 1 ${SOURCE_REPO}/dexidp/dex.git . \
  && git apply /client-groups.patch \
  && git apply /static-user-groups.patch \
  && git apply /gitlab-refresh-context.patch \
  && git apply /connector-data.patch \
  && git apply /oidc-ca-insecure.patch \
  && git apply /robots-txt.patch
RUN go build ./cmd/dex

RUN chown 64535:64535 dex
RUN chmod 0700 dex

FROM $BASE_DISTROLESS

COPY --from=artifact /dex/dex /usr/local/bin/
COPY web /web

CMD ["dex", "serve", "/etc/dex/config.docker.yaml"]
