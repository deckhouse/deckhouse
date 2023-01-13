ARG BASE_ALPINE
ARG BASE_GOLANG_19_ALPINE

FROM $BASE_GOLANG_19_ALPINE as builder

ENV GO111MODULE "on"

WORKDIR /usr/local/go/src/deckhouse-registry-webhook

COPY . /usr/local/go/src/deckhouse-registry-webhook/

RUN go mod download
ENV GOOS=linux
ENV GOARCH=amd64
ENV CGO_ENABLED=0

RUN go build \
  -v \
  -ldflags "-w -s -X 'main.BuildDatetime=$(date +\"%Y-%m-%d %H:%M:%S\")'" \
  -o webhook \
  ./main.go

FROM $BASE_ALPINE
WORKDIR /app
COPY --from=builder /usr/local/go/src/deckhouse-registry-webhook/webhook /webhook
ENTRYPOINT ["/webhook"]
