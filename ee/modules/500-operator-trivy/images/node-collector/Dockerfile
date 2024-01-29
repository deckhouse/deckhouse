ARG BASE_ALT_P101
ARG BASE_GOLANG_20_ALPINE

FROM $BASE_GOLANG_20_ALPINE AS build
ARG GOPROXY
ARG SOURCE_REPO
ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /src
RUN apk add --update --no-cache patch git
RUN git clone --depth 1 --branch v0.0.6 ${SOURCE_REPO}/aquasecurity/k8s-node-collector.git .

COPY patches/001-change-node-collector-config.patch /src
RUN patch -p1 < 001-change-node-collector-config.patch

RUN go build -ldflags '-s -w -extldflags "-static"' -o node-collector ./cmd/node-collector/main.go

FROM $BASE_ALT_P101
COPY --from=build /src/node-collector /usr/local/bin/
ENTRYPOINT [ "/usr/local/bin/node-collector" ]
