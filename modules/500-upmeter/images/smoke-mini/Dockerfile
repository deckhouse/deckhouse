FROM golang:1.14.4-alpine3.12
ADD go.mod /app/
WORKDIR /app
RUN go mod download
ADD smoke-mini.go /app/
RUN go build .

FROM alpine:3.12
COPY --from=0 /app/smoke-mini /smoke-mini
