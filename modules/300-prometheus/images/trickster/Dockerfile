ARG BASE_DISTROLESS
ARG BASE_GOLANG_16_ALPINE
ARG BASE_GOLANG_20_ALPINE

FROM $BASE_GOLANG_16_ALPINE as artifact

RUN apk add --no-cache make git
RUN git clone --depth 1 --branch v1.1.5 https://github.com/trickstercache/trickster.git /trickster
WORKDIR /trickster/

ARG GOARCH=amd64
RUN GOOS=linux GOARCH=${GOARCH} CGO_ENABLED=0 make build

FROM $BASE_GOLANG_20_ALPINE as artifact1

COPY /runner/ /app/
WORKDIR /app/

RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o runner

FROM $BASE_DISTROLESS

COPY --from=artifact /trickster/OPATH/trickster /usr/local/bin/
COPY --from=artifact1 /app/runner /usr/local/bin/

ENTRYPOINT ["runner", "<", "/etc/trickster/trickster.conf"]
