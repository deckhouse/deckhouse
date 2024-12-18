ARG BASE_GOLANG_21_ALPINE_DEV
ARG BASE_DISTROLESS
FROM $BASE_GOLANG_21_ALPINE_DEV as artifact
ARG GOPROXY
ENV GOPROXY=${GOPROXY}
ARG SOURCE_REPO
ENV SOURCE_REPO=${SOURCE_REPO}
WORKDIR /dex
RUN apk add --no-cache openssh-client
COPY patches/*.patch /patches/
RUN mkdir -p ~/.ssh && echo "StrictHostKeyChecking accept-new" > ~/.ssh/config
RUN --mount=type=ssh git clone --branch v2.41.1 --depth 1 ${SOURCE_REPO}/dexidp/dex.git . \
  && git apply /patches/*.patch --verbose

RUN CGO_ENABLED=1 GOOS=linux go build -ldflags '-s -w' -ldflags "-linkmode external -extldflags -static" -tags netgo ./cmd/dex

RUN chown 64535:64535 dex
RUN chmod 0700 dex

FROM $BASE_DISTROLESS

COPY --from=artifact /dex/dex /usr/local/bin/
COPY web /web

CMD ["dex", "serve", "/etc/dex/config.docker.yaml"]
