ARG BASE_GOLANG_ALPINE
FROM $BASE_GOLANG_ALPINE
RUN apk add --no-cache git ca-certificates gcc build-base sqlite patch make
WORKDIR /dex
COPY client-groups.patch obsolete.patch static-user-groups.patch dark-theme.patch /
RUN wget https://github.com/dexidp/dex/archive/v2.26.0.tar.gz -O - | tar -xz --strip-components=1 \
  && git apply /client-groups.patch \
  && git apply /static-user-groups.patch \
  && git apply /obsolete.patch \
  && git apply /dark-theme.patch
RUN go build ./cmd/dex

FROM dexidp/dex:v2.26.0
COPY --from=0 /dex/dex /usr/local/bin
COPY --from=0 /dex/web /web
