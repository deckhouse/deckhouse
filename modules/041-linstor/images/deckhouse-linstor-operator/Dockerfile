FROM golang:1.20-alpine3.18 as builder

RUN mkdir /go/src/deckhouse-deckhouse-linstor-operator
WORKDIR /go/src/deckhouse-linstor-operator

COPY . .
RUN go mod download
WORKDIR /go/src/deckhouse-linstor-operator/cmd
RUN GOOS=linux GOARCH=amd64 go  build -o lsopr

FROM amd64/ubuntu:jammy-20230624

WORKDIR /root/
COPY --from=builder /go/src/deckhouse-linstor-operator/cmd .

ENV SC_STABLE_REPLICAS=2
ENV SC_STABLE_QUORUM=major
ENV SC_BAD_REPICAS=1
ENV SC_BAD_QUORUM=no

CMD ["./lsopr"]