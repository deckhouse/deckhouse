ARG BASE_DISTROLESS
ARG BASE_GOLANG_20_ALPINE

FROM $BASE_GOLANG_20_ALPINE as artifact

ARG GOPROXY
ARG SOURCE_REPO

ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

RUN apk add --no-cache make git
RUN git clone --depth 1 --branch mimir-2.10.4 ${SOURCE_REPO}/grafana/mimir.git /mimir
WORKDIR /mimir/

RUN go build -o mimir ./cmd/mimir && \
    chown -R 64535:64535 /mimir/ && \
    chmod 0700 /mimir/mimir

FROM $BASE_DISTROLESS

COPY --from=artifact /mimir/mimir /bin/mimir

ENTRYPOINT ["/bin/mimir"]
