FROM golang:1.14.4-alpine3.12
ADD smoke-mini.go /
RUN go build -o /smoke-mini /smoke-mini.go && rm /smoke-mini.go

FROM alpine:3.12
COPY --from=0 /smoke-mini /smoke-mini
