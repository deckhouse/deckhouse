ARG BASE_GOLANG_19_BULLSEYE

FROM $BASE_GOLANG_19_BULLSEYE as builder

WORKDIR /go/src

COPY . .
RUN go mod download
WORKDIR /go/src/cmd
RUN GOOS=linux GOARCH=amd64 go  build -o stctrl

FROM $BASE_GOLANG_19_BULLSEYE
COPY --from=builder /go/src/cmd/stctrl /go/src/cmd/stctrl

CMD ["/go/src/cmd/stctrl"]
