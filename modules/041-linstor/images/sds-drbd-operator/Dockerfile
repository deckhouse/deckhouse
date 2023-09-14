FROM golang:1.20-alpine3.18 as builder

WORKDIR /go/src

COPY . .
RUN go mod download
WORKDIR /go/src/cmd
RUN GOOS=linux GOARCH=amd64 go  build -o stctrl

FROM amd64/ubuntu:jammy-20230624

WORKDIR /root/
COPY --from=builder /go/src/cmd .

ENV NODE_NAME=test-node

CMD ["./stctrl"]
