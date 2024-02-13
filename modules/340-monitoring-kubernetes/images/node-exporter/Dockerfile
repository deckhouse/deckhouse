ARG BASE_DISTROLESS
ARG BASE_GOLANG_20_ALPINE_DEV

FROM $BASE_GOLANG_20_ALPINE_DEV as artifact

ARG GOPROXY
ARG SOURCE_REPO

ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

RUN git clone --depth 1 --branch v1.7.0 ${SOURCE_REPO}/prometheus/node_exporter.git /node_exporter
WORKDIR /node_exporter/

RUN go mod tidy
RUN go build -ldflags="-s -w" -o node_exporter node_exporter.go && \
    chown -R 64535:64535 /node_exporter/ && \
    chmod 0700 /node_exporter/node_exporter

FROM $BASE_DISTROLESS
COPY --from=artifact /node_exporter/node_exporter /bin

EXPOSE      9100

ENTRYPOINT  [ "/bin/node_exporter" ]
