ARG BASE_GOLANG_20_ALPINE_DEV
ARG BASE_DISTROLESS
FROM $BASE_GOLANG_20_ALPINE_DEV as artifact
ARG GOPROXY
ENV GOPROXY=${GOPROXY}
ARG SOURCE_REPO
ENV SOURCE_REPO=${SOURCE_REPO}
WORKDIR /dex
COPY patches/client-groups.patch patches/static-user-groups.patch patches/gitlab-refresh-context.patch patches/connector-data.patch patches/oidc-ca-insecure.patch patches/robots-txt.patch patches/401-password-auth.patch /
RUN git clone --branch v2.35.3 --depth 1 ${SOURCE_REPO}/dexidp/dex.git . \
  && git apply /client-groups.patch \
  && git apply /static-user-groups.patch \
  && git apply /gitlab-refresh-context.patch \
  && git apply /connector-data.patch \
  && git apply /oidc-ca-insecure.patch \
  && git apply /robots-txt.patch \
  && git apply /401-password-auth.patch

RUN go get google.golang.org/grpc@v1.56.3 && \
    go mod tidy && \
    go mod vendor && \
    CGO_ENABLED=1 GOOS=linux go build -ldflags '-s -w' -ldflags "-linkmode external -extldflags -static" -tags netgo ./cmd/dex

RUN chown 64535:64535 dex
RUN chmod 0700 dex

FROM $BASE_DISTROLESS

COPY --from=artifact /dex/dex /usr/local/bin/
COPY web /web

CMD ["dex", "serve", "/etc/dex/config.docker.yaml"]
