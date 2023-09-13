FROM golang:1.20-alpine3.18 as builder

RUN mkdir /go/src/d8-sds-drbd-operator
WORKDIR /go/src/d8-sds-drbd-operator

COPY . .
RUN go mod download
WORKDIR /go/src/d8-sds-drbd-operator/cmd
RUN GOOS=linux GOARCH=amd64 go  build -o lsopr

FROM amd64/ubuntu:jammy-20230624

WORKDIR /root/
COPY --from=builder /go/src/d8-sds-drbd-operator/cmd .

CMD ["./lsopr"]
