ARG BASE_GOLANG_21_ALPINE
ARG BASE_DISTROLESS
FROM $BASE_GOLANG_21_ALPINE as artifact
COPY /src/ /app/
WORKDIR /app

ARG GOPROXY
ARG SOURCE_REPO

ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

RUN go build -ldflags="-s -w" -o alerts-receiver && \
    chown -R 64535:64535 /app/ && \
    chmod 0700 /app/alerts-receiver

FROM $BASE_DISTROLESS
COPY --from=artifact /app/alerts-receiver /alerts-receiver
ENTRYPOINT [ "/alerts-receiver" ]
