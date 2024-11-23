ARG BASE_ALT
ARG BASE_GOLANG_22_ALPINE_DEV

FROM $BASE_GOLANG_22_ALPINE_DEV AS build
ARG GOPROXY
ARG SOURCE_REPO
ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /src
RUN apk add --no-cache openssh-client
RUN mkdir -p ~/.ssh && echo "StrictHostKeyChecking accept-new" > ~/.ssh/config
RUN --mount=type=ssh git clone --depth 1 --branch v0.3.1 ${SOURCE_REPO}/aquasecurity/k8s-node-collector.git .

RUN go build -ldflags '-s -w -extldflags "-static"' -o node-collector ./cmd/node-collector/main.go

FROM $BASE_ALT
COPY --from=build /src/node-collector /usr/local/bin/
ENTRYPOINT [ "/usr/local/bin/node-collector" ]
