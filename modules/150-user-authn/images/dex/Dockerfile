ARG BASE_GOLANG_ALPINE
FROM $BASE_GOLANG_ALPINE
RUN apk add --no-cache git ca-certificates gcc build-base sqlite patch make curl
WORKDIR /dex
COPY client-groups.patch obsolete.patch static-user-groups.patch /
RUN wget https://github.com/dexidp/dex/archive/v2.28.1.tar.gz -O - | tar -xz --strip-components=1 \
  && git apply /client-groups.patch \
  && git apply /static-user-groups.patch \
  && git apply /obsolete.patch
RUN go build ./cmd/dex

FROM dexidp/dex:v2.28.1
COPY --from=0 /dex/dex /usr/local/bin
COPY --from=0 /dex/web /web
