ARG BASE_ALPINE
ARG BASE_GOLANG_16_ALPINE

FROM $BASE_GOLANG_16_ALPINE as artifact
WORKDIR /src/
RUN apk add git patch
RUN git clone -b "v0.11.0" --single-branch https://github.com/metallb/metallb
WORKDIR /src/metallb
COPY patches/dont-announce-from-annotated-nodes.patch ./

RUN patch -p1 < dont-announce-from-annotated-nodes.patch
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o speaker-bin ./speaker

FROM $BASE_ALPINE
COPY --from=artifact /src/metallb/speaker-bin /speaker
ENTRYPOINT ["/speaker"]
