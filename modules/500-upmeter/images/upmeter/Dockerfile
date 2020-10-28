ARG BASE_ALPINE
ARG BASE_GOLANG_ALPINE
FROM $BASE_GOLANG_ALPINE as builder
RUN apk add --update gcc musl-dev jq-dev oniguruma-dev
ADD /go.mod /app/go.mod
WORKDIR /app
RUN go mod download
ADD / /app
RUN go build -ldflags "-linkmode external -extldflags '-static'" -o /upmeter ./cmd/upmeter

FROM $BASE_ALPINE
COPY --from=builder /upmeter /upmeter
