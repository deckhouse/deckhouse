ARG BASE_GOLANG_19_ALPINE
ARG BASE_DISTROLESS
FROM $BASE_GOLANG_19_ALPINE as artifact
COPY /src/ /app/
WORKDIR /app
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o alerts-receiver && \
    chown -R 64535:64535 /app/ && \
    chmod 0700 /app/alerts-receiver

FROM $BASE_DISTROLESS
COPY --from=artifact /app/alerts-receiver /alerts-receiver
ENTRYPOINT [ "/alerts-receiver" ]
