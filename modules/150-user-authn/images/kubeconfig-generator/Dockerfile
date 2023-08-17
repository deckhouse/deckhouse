ARG BASE_DISTROLESS
ARG BASE_GOLANG_20_ALPINE
FROM $BASE_GOLANG_20_ALPINE as artifact
ARG SOURCE_REPO
ENV SOURCE_REPO=${SOURCE_REPO}

RUN apk add bash git make

WORKDIR /src

ADD already_logged.patch /

RUN git clone ${SOURCE_REPO}/mintel/dex-k8s-authenticator.git . && \
  git checkout 378a39dd93bed9f56a5a1b1a799a208c61ead83f && \
  git apply --whitespace=fix /already_logged.patch && \
  go mod download && \
  mkdir -p /app/app/bin && \
  GOPROXY=https://proxy.golang.org CGO_ENABLED=0 GOOS=linux go build -ldflags '-s -w' -o /app/app/bin/dex-k8s-authenticator .

RUN chown 64535:64535 /app/app/bin/dex-k8s-authenticator
RUN chmod 0700 /app/app/bin/dex-k8s-authenticator

# set up nsswitch.conf for Go's "netgo" implementation
# Go stdlib completely ignores /etc/hosts file without it
# https://github.com/moby/moby/issues/34544
RUN echo "hosts: files dns" > /etc/nsswitch.conf

FROM $BASE_DISTROLESS

COPY --from=artifact /app /
COPY --from=artifact /etc/nsswitch.conf /etc/
COPY --from=artifact /src/html /app/html
COPY --from=artifact /src/templates /app/templates
COPY --from=artifact /src/entrypoint.sh /

WORKDIR /app

ENTRYPOINT ["/dex-k8s-authenticator"]
