FROM golang:1.20-alpine3.18 as builder

RUN mkdir /go/src/d8-linstor-operator
WORKDIR /go/src/deckhouse-linstor-operator

COPY . .
RUN go mod download
WORKDIR /go/src/d8-linstor-operator/cmd
RUN GOOS=linux GOARCH=amd64 go  build -o lsopr

FROM amd64/ubuntu:jammy-20230624

WORKDIR /root/
COPY --from=builder /go/src/d8-linstor-operator/cmd .

CMD ["./lsopr"]
