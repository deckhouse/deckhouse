ARG BASE_GOLANG_19_ALPINE
ARG BASE_SCRATCH
FROM $BASE_GOLANG_19_ALPINE as artifact
ADD go.mod /app/
ADD go.sum /app/
WORKDIR /app
ADD alerts-receiver-webhook.go /app/
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o alerts-receiver-webhook alerts-receiver-webhook.go && chmod +x ./alerts-receiver-webhook

FROM $BASE_SCRATCH
COPY --from=artifact /app/alerts-receiver-webhook /alerts-receiver-webhook
ENTRYPOINT [ "/alerts-receiver" ]
