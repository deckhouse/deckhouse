ARG BASE_GOLANG_19_BULLSEYE

FROM $BASE_GOLANG_19_BULLSEYE

WORKDIR /go/src

COPY . .
RUN go mod download
WORKDIR /go/src/cmd
RUN GOOS=linux GOARCH=amd64 go  build -o stctrl

CMD ["./stctrl"]
