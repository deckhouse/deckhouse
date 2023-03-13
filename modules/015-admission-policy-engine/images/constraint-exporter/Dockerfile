ARG BASE_ALPINE
ARG BASE_GOLANG_18_BULLSEYE

FROM $BASE_GOLANG_18_BULLSEYE as builder
WORKDIR /app
COPY . .
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o constraint_exporter .


FROM $BASE_ALPINE
COPY --from=builder /app/constraint_exporter /app/constraint_exporter
CMD ["/app/constraint_exporter"]
