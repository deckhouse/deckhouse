ARG BASE_DISTROLESS
ARG BASE_GOLANG_20_ALPINE

FROM $BASE_GOLANG_20_ALPINE as builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o constraint_exporter .

RUN chown 64535:64535 constraint_exporter
RUN chmod 0700 constraint_exporter

FROM $BASE_DISTROLESS
COPY --from=builder /app/constraint_exporter /app/constraint_exporter
CMD ["/app/constraint_exporter"]
