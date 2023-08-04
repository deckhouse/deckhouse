ARG BASE_DISTROLESS
ARG BASE_GOLANG_20_ALPINE
ARG SOURCE_REPO

FROM $BASE_GOLANG_20_ALPINE AS build
ENV SOURCE_REPO=${SOURCE_REPO}
WORKDIR /src
RUN apk add --update --no-cache patch git
RUN git clone --depth 1 --branch v0.0.6 ${SOURCE_REPO}/aquasecurity/k8s-node-collector.git .

COPY patches/001-change-node-collector-config.patch /src
RUN patch -p1 < 001-change-node-collector-config.patch

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags '-s -w -extldflags "-static"' -o node-collector ./cmd/node-collector/main.go

RUN chown 64535:64535 node-collector
RUN chmod 0700 node-collector

FROM $BASE_DISTROLESS
COPY --from=build /src/node-collector /usr/local/bin/
ENTRYPOINT [ "/usr/local/bin/node-collector" ]
