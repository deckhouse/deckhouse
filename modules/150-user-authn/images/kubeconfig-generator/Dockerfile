ARG BASE_DISTROLESS
ARG BASE_GOLANG_20_ALPINE
FROM $BASE_GOLANG_20_ALPINE as artifact
ARG GOPROXY
ARG SOURCE_REPO

ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

RUN apk add bash git make

WORKDIR /src

ADD already_logged.patch /

RUN git clone ${SOURCE_REPO}/mintel/dex-k8s-authenticator.git . && \
  git checkout 378a39dd93bed9f56a5a1b1a799a208c61ead83f && \
  git apply --whitespace=fix /already_logged.patch && \
  go mod edit -go=1.20 && \
  go mod tidy && \
  go get -u golang.org/x/crypto@v0.15.0 && \
  go get -u golang.org/x/net@v0.17.0 && \
  go get -u golang.org/x/text@v0.14.0 && \
  go get -u gopkg.in/yaml.v2@v2.4.0 && \
  mkdir -p /app/app/bin && \
  go build -ldflags '-s -w' -o /app/app/bin/dex-k8s-authenticator .

RUN chown 64535:64535 /app/app/bin/dex-k8s-authenticator
RUN chmod 0700 /app/app/bin/dex-k8s-authenticator

FROM $BASE_DISTROLESS

COPY --from=artifact /app /
COPY --from=artifact /etc/nsswitch.conf /etc/
COPY --from=artifact /src/html /app/html
COPY --from=artifact /src/templates /app/templates
COPY --from=artifact /src/entrypoint.sh /

WORKDIR /app

ENTRYPOINT ["/dex-k8s-authenticator"]
