ARG BASE_GOLANG_19_ALPINE
ARG BASE_SCRATCH
FROM $BASE_GOLANG_19_ALPINE as artifact
ADD go.mod /app
ADD go.sum /app
WORKDIR /app
ADD alerts-receiver.go /app
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o alerts-receiver alerts-receiver.go && chmod +x ./alerts-receiver

FROM $BASE_SCRATCH
COPY --from=artifact /app/alerts-receiver /alerts-receiver
CMD /alerts-receiver
