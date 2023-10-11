ARG BASE_GOLANG_19_BULLSEYE

FROM $BASE_GOLANG_19_BULLSEYE as builder

WORKDIR /go/src

ADD go.mod .
ADD go.sum .

RUN go mod download

COPY . .

WORKDIR /go/src/cmd
RUN GOOS=linux GOARCH=amd64 go build -o stctrl

FROM --platform=linux/amd64 $BASE_GOLANG_19_BULLSEYE
COPY --from=builder /go/src/cmd/stctrl /go/src/cmd/stctrl

CMD ["/go/src/cmd/stctrl"]
