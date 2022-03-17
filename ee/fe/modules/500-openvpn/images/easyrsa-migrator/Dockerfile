ARG BASE_ALPINE
ARG BASE_GOLANG_17_ALPINE

FROM $BASE_GOLANG_17_ALPINE AS build
WORKDIR /app
COPY easyrsa-migrator.go go.mod go.sum /app/
RUN go build .

FROM $BASE_ALPINE
WORKDIR /app
RUN apk add --no-cache bash openssl openvpn
COPY --from=build /app/easyrsa-migrator /app/
ENV LANG=C.UTF-8
